package zmap3base

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"sort"
)

// BaseStore：只读 base 段池（LP/HP 共用 initRangeData）。
//
// 约定：initRangeData 以“段”为单位拼接存储，每段的第一个元素是 header：
//   - header.Range.Begin 存 segLen（整段长度，包含 header 自己）
//   - header.Range.End：
//   - LP 段：存该 cell 的地形高度（通常 >= payload 的最大 End）
//   - HP 段：固定写 0（仅用于识别/跳过 header）
//   - payload 从 seg[1:] 开始（按 Begin 升序）
//
// rootIdx：指向某段 header 在 initRangeData 中的起点索引（rootIdx>0）。
// - rootIdx==0：空引用（占位）
// - rootIdx>0：有效段起点（是 encoded base root）
//
// encoded root 分三类：
//   - r<=0：空（NilIdx=-1 或 0 空引用）
//   - 0<r<BaseThreshold：base 段 rootIdx（也就是 BaseStore 的 rootIdx）
//   - r>=BaseThreshold：dirty RBTree root（EncodeDirtyRoot）
//
// BaseStore 只处理 base 段；对 dirty encoded root 传入时会返回 nil（安全失败）。
type BaseStore struct {
	// 段池（LP/HP 段共享池）
	initRangeData []RichRange

	// 每段数据对应的引用计数（段起点 rootIdx -> refCount）
	// 说明：只对段起点 key 有意义；非段起点的 key 即便存在也不应被使用。
	rootCount map[int32]uint16

	// bucketData：构建期 hash buckets 的序列化缓存（卸载/调试等场景用）
	bucketData []byte
}

// GetSlice 返回 rootIdx 指向的整段切片（包含 header）。
// - rootIdx<=0 或越界 => nil
// - 该段的总长度存放在 slice[0].Begin
func (b *BaseStore) GetSlice(rootIdx int32) []RichRange {
	if rootIdx <= 0 || rootIdx >= int32(len(b.initRangeData)) {
		return nil
	}
	segLen := int(b.initRangeData[rootIdx].Begin)
	if segLen <= 0 {
		return nil
	}
	hi := int(rootIdx) + segLen
	if hi > len(b.initRangeData) {
		// 数据损坏保护：避免 panic
		return nil
	}
	return b.initRangeData[rootIdx:hi]
}

// GetPayload 返回段的 payload（跳过 header），以及 header 本身。
// - hdr 是 seg[0]（Begin=段长度，End=LP 地形高度 / HP 为 0）
// - payload 是 seg[1:]
func (b *BaseStore) GetPayload(rootIdx int32) (hdr RichRange, payload []RichRange, ok bool) {
	seg := b.GetSlice(rootIdx)
	if len(seg) == 0 {
		return RichRange{}, nil, false
	}
	hdr = seg[0]
	if len(seg) > 1 {
		payload = seg[1:]
	}
	return hdr, payload, true
}

// GetSliceEncoded： 输入获取 base 段。
// - 若 encodedRoot 不是 baseRoot（例如 dirtyRoot），返回 nil。
func (b *BaseStore) GetSliceEncoded(encodedRoot int32) []RichRange {
	if !IsBaseEncodedRoot(encodedRoot) {
		return nil
	}
	return b.GetSlice(DecodeBaseRoot(encodedRoot))
}

// GetPayloadEncoded： 输入获取 base 段 payload。
// - 若 encodedRoot 不是 baseRoot（例如 dirtyRoot），返回 ok=false。
func (b *BaseStore) GetPayloadEncoded(encodedRoot int32) (hdr RichRange, payload []RichRange, ok bool) {
	if !IsBaseEncodedRoot(encodedRoot) {
		return RichRange{}, nil, false
	}
	return b.GetPayload(DecodeBaseRoot(encodedRoot))
}

// Buckets 反序列化返回构建期的 hash buckets（用于卸载/调试等场景）。
// 若没有 bucketData，则返回 nil,nil。
func (b *BaseStore) Buckets() (map[uint64][]int32, error) {
	if len(b.bucketData) == 0 {
		return nil, nil
	}
	return unmarshBuckets(b.bucketData)
}

// ======================= Build =======================

// BuildBaseStoreFromSlices：由外部 slices 构造 BaseStore（LP + HP 的段池共享）
//
// lpPerCell: len=FastGridCellNum，每项为该 cell 的 LP RangeList（index0 必须是“地形起点 rr”，Begin 原本为 0）
// hpPerCell: len=FastGridCellNum，每项为 [SecondaryTileNum][]RichRange（历史 16；当前只用 activeSub）
//

// BuildGridRBDataFromSlices：构造一个完整 GridRBData（含每 cell 的 RootNode/HP spans 映射 + BaseStore 段池）
func BuildGridRBDataFromSlices(minX, minY uint16, lpPerCell [][]RichRange, hpPerCell [][SecondaryTileNum][]RichRange) (*GridRBData, error) {
	if len(lpPerCell) != FastGridCellNum {
		return nil, errors.New("BuildGridRBDataFromSlices: lpPerCell len != FastGridCellNum")
	}
	if hpPerCell != nil && len(hpPerCell) != FastGridCellNum {
		return nil, errors.New("BuildGridRBDataFromSlices: hpPerCell len != FastGridCellNum")
	}

	activeSub := SecondaryTileNum

	var cells [FastGridCellNum]RichRangeSetData

	// 共享池：0 号位永远占位，使 rootIdx=0 永远表示“空引用”
	initRangeData := make([]RichRange, 0, 1024)
	initRangeData = append(initRangeData, RichRange{})

	allBuckets := make(map[uint64][]int32, 128)
	rootCount := make(map[int32]uint16, 128)

	checkAndAdd := func(rrs []RichRange) (int32, error) {
		if len(rrs) == 0 {
			return 0, errors.New("checkAndAdd: empty rrs")
		}
		if int(rrs[0].Begin) != len(rrs) {
			return 0, errors.New("checkAndAdd: rrs[0].Begin must store segLen")
		}

		h := sliceHash(rrs)
		curIdx := int32(len(initRangeData))

		// 保护护栏：base 段 rootIdx 不得撞 BaseThreshold
		if curIdx >= BaseThreshold {
			return 0, errors.New("checkAndAdd: base rootIdx reached BaseThreshold; would collide with dirty encoding")
		}

		if cans, ok := allBuckets[h]; ok {
			for _, si := range cans {
				if si <= 0 || int(si) >= len(initRangeData) {
					continue
				}
				rrsLen := int(initRangeData[si].Begin)
				if rrsLen <= 0 {
					continue
				}
				hi := int(si) + rrsLen
				if hi > len(initRangeData) {
					continue
				}
				if sliceEqual(initRangeData[si:hi], rrs) {
					rootCount[si]++
					return si, nil
				}
			}
		}
		allBuckets[h] = append(allBuckets[h], curIdx)
		rootCount[curIdx]++
		initRangeData = append(initRangeData, rrs...)
		return curIdx, nil
	}

	// -------- LP --------
	for cell := 0; cell < FastGridCellNum; cell++ {
		src := lpPerCell[cell]
		if len(src) == 0 {
			// 保底：为空时仍写入默认 terrain header，避免 RootNode=0
			terrain := MakeRange(0, 0, TextureMaterBase, 0)
			seg := []RichRange{terrain}
			seg[0].Range = Range{uint16(len(seg)), terrain.End}
			rootIdx, err := checkAndAdd(seg)
			if err != nil {
				return nil, err
			}
			cells[cell].RootNode = EncodeBaseRoot(rootIdx)
			continue
		}

		terrain := src[0]
		if terrain.Begin != 0 || (terrain.Accessory.Texture&TextureMaterBase) == 0 {
			return nil, errors.New("BuildGridRBDataFromSlices: lpPerCell must have terrain at src[0]")
		}

		payload := src[1:] // terrain 不进 payload（由 header 复原）
		seg := make([]RichRange, 1+len(payload))

		// header：用 terrain 的 Accessory；Begin=段长；End=terrainEnd（允许 0）
		seg[0] = terrain
		seg[0].Range = Range{uint16(len(seg)), terrain.End}
		copy(seg[1:], payload)

		rootIdx, err := checkAndAdd(seg)
		if err != nil {
			return nil, err
		}
		cells[cell].RootNode = EncodeBaseRoot(rootIdx)
	}

	// -------- HP --------
	if hpPerCell != nil {
		for cell := 0; cell < FastGridCellNum; cell++ {
			localSpans := make([][]RichRange, 0, activeSub)
			localBuckets := make(map[uint64][]uint8, activeSub)

			findOrAddLocal := func(rrs []RichRange) (uint8, error) {
				if len(rrs) == 0 {
					return 0, errors.New("internal: empty rrs in findOrAddLocal")
				}
				h := sliceHash(rrs)
				if cans, ok := localBuckets[h]; ok {
					for _, si := range cans {
						if int(si) < len(localSpans) && sliceEqual(localSpans[si], rrs) {
							return si, nil
						}
					}
				}
				if len(localSpans) >= activeSub {
					return 0, errors.New("cell HP span overflow: >HPActiveSub unique HP slices in one cell")
				}
				si := uint8(len(localSpans))
				localSpans = append(localSpans, rrs)
				localBuckets[h] = append(localBuckets[h], si)
				return si, nil
			}

			// 只处理 activeSub
			for sub := 0; sub < activeSub; sub++ {
				final := hpPerCell[cell][sub]
				if len(final) == 0 {
					continue
				}

				if cells[cell].HighPrecision == nil {
					cells[cell].HighPrecision = &HighPrecisionColumn{}
				}
				hpCell := cells[cell].HighPrecision

				si, err := findOrAddLocal(final)
				if err != nil {
					return nil, err
				}
				hpCell.Same.Set(sub, si)
				hpCell.Has |= uint16(1) << uint(sub)
			}

			if len(localSpans) == 0 {
				continue
			}

			cells[cell].HighPrecision.Spans = make([]int32, len(localSpans))

			for i := 0; i < len(localSpans); i++ {
				payload := localSpans[i]
				if len(payload) == 0 {
					continue
				}

				seg := make([]RichRange, len(payload)+1)
				seg[0] = RichRange{Range: Range{uint16(len(seg)), 0}} // HP header.End=0
				copy(seg[1:], payload)

				rootIdx, err := checkAndAdd(seg)
				if err != nil {
					return nil, err
				}
				// 保护：显式 encode（baseRoot 编码为自身）
				cells[cell].HighPrecision.Spans[i] = EncodeBaseRoot(rootIdx)
			}
		}
	}

	bucketData, err := marshBuckets(allBuckets)
	if err != nil {
		return nil, err
	}

	grid := NewGridRBData(minX, minY, 0)
	grid.InitBase(BaseStore{initRangeData: initRangeData, rootCount: rootCount, bucketData: bucketData})
	for i := 0; i < FastGridCellNum; i++ {
		grid.CellByIdx(i).RootNode = cells[i].RootNode
		grid.CellByIdx(i).HighPrecision = cells[i].HighPrecision
	}

	return grid, nil
}

func marshBuckets(buckets map[uint64][]int32) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	wr := NewBinWriter(buffer, true)
	wr.WriteUint32(uint32(len(buckets)))
	for h, sis := range buckets {
		wr.WriteUint64(h)
		wr.WriteUint32(uint32(len(sis)))
		for _, si := range sis {
			wr.WriteInt32(si)
		}
	}
	wr.Flush()
	return buffer.Bytes(), nil
}

func unmarshBuckets(data []byte) (map[uint64][]int32, error) {
	buffer := bytes.NewBuffer(data)
	br := NewBinReader(buffer, true)
	bucketLen := br.ReadUint32()
	buckets := make(map[uint64][]int32, bucketLen)
	for i := uint32(0); i < bucketLen; i++ {
		h := br.ReadUint64()
		sisLen := br.ReadUint32()
		sis := make([]int32, sisLen)
		for j := uint32(0); j < sisLen; j++ {
			sis[j] = br.ReadInt32()
		}
		buckets[h] = sis
	}
	return buckets, nil
}

// rangeQueryBaseSlice：对 base 段（通常来自 BaseStore.GetSlice/GetSliceEncoded，含 header）做 range query。
// 语义：回调所有与 rq 有交集的 rr（按 Begin 升序），fn 返回 false 可早停。
//
// 注意：base overlays 允许“不同 config 重叠”，End 并不单调，不能用“向左回退直到 End<=qb”的办法找起点，否则会漏掉很早开始但跨过 qb 的长区间。
// 这里采用：找到 Begin < qe 的前缀范围，然后线性检查 End > qb（正确且仍有 early break）。
//
// header 跳过规则：
// - 若 rrs[0] 是 header（rrs[0].Begin==len(rrs) 且满足 LP/HP header 形态），则跳过 rrs[0]。
func rangeQueryBaseSlice(rrs []RichRange, rq Range, fn func(rr RichRange) bool) {
	if len(rrs) == 0 || rq.End-rq.Begin == 0 {
		return
	}

	if len(rrs) > 0 && int(rrs[0].Begin) == len(rrs) {
		rrs = rrs[1:]
		if len(rrs) == 0 {
			return
		}
	}

	qb := rq.Begin
	qe := rq.End

	// 只需要扫描 Begin < qe 的前缀（Begin 升序）
	end := sort.Search(len(rrs), func(i int) bool {
		return rrs[i].Begin >= qe
	})

	for i := 0; i < end; i++ {
		rr := rrs[i]
		// 相交判断：End > qb && Begin < qe
		if rr.End > qb {
			if !fn(rr) {
				return
			}
		}
	}
}

func sliceEqual(a, b []RichRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i].Range != b[i].Range {
			return false
		}
		if a[i].Accessory.IntoUint64() != b[i].Accessory.IntoUint64() {
			return false
		}
	}
	return true
}

func sliceHash(rrs []RichRange) uint64 {
	h := fnv.New64a()
	var tmp [16]byte
	for i := 0; i < len(rrs); i++ {
		// Range(uint32) + Accessory(uint64)
		binary.LittleEndian.PutUint16(tmp[0:2], rrs[i].Range.Begin)
		binary.LittleEndian.PutUint16(tmp[2:4], rrs[i].Range.End)
		binary.LittleEndian.PutUint64(tmp[4:12], rrs[i].Accessory.IntoUint64())
		_, _ = h.Write(tmp[:12])
	}
	return h.Sum64() ^ uint64(len(rrs))
}
