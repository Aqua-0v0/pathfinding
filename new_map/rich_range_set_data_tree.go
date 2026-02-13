package zmap3base

// ===================== Root 多语义编码 =====================
//
// RootNode / HP span root 的取值空间被拆成两段：
//   - r <= 0              : 无效/空（Nil=-1 或 0 作为保留值）
//   - 0 < r < BaseThreshold: baseRootIdx（BaseStore.initRangeData 的段起点下标）
//   - r >= BaseThreshold  : dirtyRBRoot（NodePool.nodes 的 root 节点索引），编码为 BaseThreshold + nodeIdx
//
// 用一个 int32 同时表达 base-slice root 与 dirty RBTree root。
const BaseThreshold int32 = 1 << 30 // 1,073,741,824（足够大）

func IsNilEncodedRoot(r int32) bool { return r <= 0 }
func IsBaseEncodedRoot(r int32) bool {
	return r > 0 && r < BaseThreshold
}
func IsDirtyEncodedRoot(r int32) bool { return r >= BaseThreshold }

func EncodeBaseRoot(rootIdx int32) int32 {
	// 约定：rootIdx 必须 >0（0 作为“空引用保留值”）
	return rootIdx
}
func DecodeBaseRoot(r int32) int32 { return r }

func EncodeDirtyRoot(nodeIdx int32) int32 {
	// 约定：nodeIdx 必须 >=0
	return BaseThreshold + nodeIdx
}
func DecodeDirtyRoot(r int32) int32 { return r - BaseThreshold }

// ===================== RichRangeSetData =====================

// RichRangeSetData ：一个基础格点上的 Range 数据集合（cell 级）。
//
// - Terrain 已经合并进 base 段/dirty 树语义里（外层不再单独持 Terrain 字段）。
// - RootNode：多语义编码（见上）。
// - HighPrecision：HP 子格点覆盖（span 共享 + Same 映射 + Has bitset）。
type RichRangeSetData struct {
	//Terrain       RichRange
	RootNode      int32
	HighPrecision *HighPrecisionColumn
	Climate       Climate // 当前单位格的气候/地形（水/陆）
}

// Same：16 个子格点映射压缩（用 64bit 放 16 个 4-bit index）
// 第 i 个子格点使用第 (i*4) 位开始的 nibble。
type Same uint64

func (s Same) Get(i int) uint8 {
	shift := uint(i * 4)
	return uint8((uint64(s) >> shift) & 0xF)
}

func (s *Same) Set(i int, v uint8) {
	shift := uint(i * 4)
	mask := uint64(0xF) << shift
	*s = Same((uint64(*s) &^ mask) | ((uint64(v) & 0xF) << shift))
}

// SubIdxFromPoint2d：把 Point2d 映射成 SecondaryAccuracy×SecondaryAccuracy 子格点 subIdx（0..SecondaryAccuracy*SecondaryAccuracy-1）
//
// 约定：Offset==0 表示低精点；Offset in [1..SecondaryAccuracy] 表示 HP 子格点。
// 映射方向：x-major（与老方案一致）
// - SecondaryAccuracy==4：sub = sx*4 + sy  => (sx<<2)|sy
func SubIdxFromPoint2d(p Point2d) (subIdx int, ok bool) {
	if p.XOffset == 0 || p.YOffset == 0 {
		return -1, false
	}
	if p.XOffset > uint8(SecondaryAccuracy) || p.YOffset > uint8(SecondaryAccuracy) {
		return -1, false
	}

	sx := int(p.XOffset - 1)
	sy := int(p.YOffset - 1)

	return (sx << 2) | sy, true

}

func SubIdxToOffset(subIdx int) (xOff, yOff uint8) {
	sx := subIdx >> 2
	sy := subIdx & 3
	return uint8(sx + 1), uint8(sy + 1)
}

// HighPrecisionColumn：HP 子格点覆盖结构（span 共享 + Same 映射）。
//
// Has：bitset，表示 subIdx 是否存在“覆盖”。
// - bit=0：没有覆盖（查询可回落 baseHP）
// - bit=1：有覆盖（不回落 baseHP；覆盖内容由 span root 决定）
//
// Same：subIdx -> spanIndex（0..15）
// Spans：spanIndex -> encodedRoot（多语义编码：baseRootIdx 或 dirtyRoot）
type HighPrecisionColumn struct {
	Has  uint16
	Same Same
	// spanIndex -> root
	Spans []int32
}

func (hp *HighPrecisionColumn) ensureInit() {
	if hp.Spans != nil {
		return
	}
	// 直接从 0 开始分配 span，不再预留 0。
	hp.Spans = make([]int32, 0, SecondaryTileNum)
	hp.Has = 0
	hp.Same = 0
}

// HasSpan：subIdx 是否存在覆盖（由 Has bitset 决定）。
func (hp *HighPrecisionColumn) HasSpan(subIdx int) bool {
	if hp == nil {
		return false
	}
	if subIdx < 0 || subIdx >= SecondaryTileNum {
		return false
	}
	return (hp.Has & (1 << uint(subIdx))) != 0
}

func (hp *HighPrecisionColumn) setHas(subIdx int, on bool) {
	if subIdx < 0 || subIdx >= SecondaryTileNum {
		return
	}
	mask := uint16(1 << uint(subIdx))
	if on {
		hp.Has |= mask
	} else {
		hp.Has &^= mask
	}
}

// SpanIdxOf：返回 subIdx 的 spanIndex（仅当 HasSpan=true 时有效）。
func (hp *HighPrecisionColumn) SpanIdxOf(subIdx int) uint8 {
	if hp == nil {
		return 0
	}
	hp.ensureInit()
	return hp.Same.Get(subIdx)
}

func (hp *HighPrecisionColumn) rootPtrOf(subIdx int) *int32 {
	if hp == nil || !hp.HasSpan(subIdx) {
		return nil
	}
	hp.ensureInit()
	span := int(hp.Same.Get(subIdx))
	if span < 0 || span >= len(hp.Spans) {
		return nil
	}
	return &hp.Spans[span]
}

func (hp *HighPrecisionColumn) RootPtrOf(subIdx int) *int32 { return hp.rootPtrOf(subIdx) }

// refCount：统计当前有多少个 subIdx 引用了 spanIdx（只统计 Has=1 的 subIdx）。
func (hp *HighPrecisionColumn) refCount(spanIdx uint8) int {
	if hp == nil {
		return 0
	}
	cnt := 0
	for sub := 0; sub < SecondaryTileNum; sub++ {
		if (hp.Has & (1 << uint(sub))) == 0 {
			continue
		}
		if hp.Same.Get(sub) == spanIdx {
			cnt++
		}
	}
	return cnt
}

// replaceSpanIndex：把所有引用 oldIdx 的 subIdx 替换为 newIdx（只替换 Has=1 的 subIdx）。
func (hp *HighPrecisionColumn) replaceSpanIndex(oldIdx, newIdx uint8) {
	if hp == nil {
		return
	}
	for sub := 0; sub < SecondaryTileNum; sub++ {
		if (hp.Has & (1 << uint(sub))) == 0 {
			continue
		}
		if hp.Same.Get(sub) == oldIdx {
			hp.Same.Set(sub, newIdx)
		}
	}
}

// tryReclaimEmptySpan：若 spanIdx 对应 root 为空且无人引用，则回收该 span（swap-remove）。
//
// 说明：这里把 “空” 判定为 root==NilIdx()；如果你后续把“空 root 判定”改成 0，
//
//	那么需要一起改动这里的判空条件（以及 TreeOps/tree 保存规则）。
func (hp *HighPrecisionColumn) tryReclaimEmptySpan(spanIdx uint8) {
	if hp == nil {
		return
	}
	hp.ensureInit()
	if int(spanIdx) < 0 || int(spanIdx) >= len(hp.Spans) {
		return
	}
	if !IsNilEncodedRoot(hp.Spans[spanIdx]) {
		return
	}
	if hp.refCount(spanIdx) != 0 {
		return
	}

	last := uint8(len(hp.Spans) - 1)
	if spanIdx == last {
		hp.Spans = hp.Spans[:last]
		return
	}

	// swap-remove：把 last 挪到 spanIdx
	hp.Spans[spanIdx] = hp.Spans[last]
	hp.Spans = hp.Spans[:last]
	// 更新 Same：把所有引用 last 的 subIdx 改为 spanIdx
	hp.replaceSpanIndex(last, spanIdx)
}

// clearSub：清理 subIdx 的覆盖（允许回落 baseHP）。
// - 清 Has bit
// - 重置 Same nibble（非必须，但有利于调试）
// - 若对应 span 变为无人引用且为空，则回收 span
func (hp *HighPrecisionColumn) clearSub(subIdx int) {
	if hp == nil || !hp.HasSpan(subIdx) {
		return
	}
	hp.ensureInit()
	oldSpan := hp.Same.Get(subIdx)

	hp.setHas(subIdx, false)
	hp.Same.Set(subIdx, 0)
	hp.tryReclaimEmptySpan(oldSpan)

	if hp.Has == 0 {
		hp.Spans = nil
		hp.Same = 0
	}
}

// allocSpan：分配一个新的 spanIndex（0..15）。
// 若已满，会尝试复用“无人引用”的 span（其 root 会被重置为 NilIdx）。
func (hp *HighPrecisionColumn) allocSpan() uint8 {
	hp.ensureInit()
	if len(hp.Spans) < SecondaryTileNum {
		hp.Spans = append(hp.Spans, NilIdx()) // encoded nil
		return uint8(len(hp.Spans) - 1)
	}
	// 已到 active 上限：尝试复用无人引用的 span
	for i := 0; i < SecondaryTileNum; i++ {
		idx := uint8(i)
		if idx >= uint8(len(hp.Spans)) {
			continue
		}
		if hp.refCount(idx) == 0 {
			hp.Spans[idx] = NilIdx()
			return idx
		}
	}
	panic("HighPrecision spans exhausted (>=16) while allocating new span")
}

// ===================== TreeOps（只负责 dirty RBTree）=====================

// TreeOps：把 pool 注入进来（pool 挂在 Env/Grid 上）
type TreeOps struct {
	pool *NodePool
}

func NewTreeOps(pool *NodePool) TreeOps { return TreeOps{pool: pool} }

// TreeFromEncodedRoot：仅当 encodedRoot 是 dirtyRoot 时返回可操作树；否则返回空树。
// 注意：baseRoot 不应该直接用 RBTree 修改（必须先 materialize 为 dirty）。
func (op TreeOps) TreeFromEncodedRoot(encodedRoot int32) RichRangeTree {
	t := NewRichRangeTree(op.pool)
	if !IsDirtyEncodedRoot(encodedRoot) {
		return t
	}
	r := DecodeDirtyRoot(encodedRoot)
	if op.pool == nil || r < 0 || int(r) >= len(op.pool.nodes) {
		return NewRichRangeTree(op.pool) // empty
	}
	t.SetRoot(r)
	//t.SetRoot(DecodeDirtyRoot(encodedRoot))
	return t
}

// SaveDirtyTree：把 t 的 root 写回 encodedRoot（编码为 dirtyRoot）；空树写 NilIdx()。
func (op TreeOps) SaveDirtyTree(encodedRoot *int32, t RichRangeTree) {
	if encodedRoot == nil {
		return
	}
	r := t.Root()
	if r < 0 {
		*encodedRoot = NilIdx()
		return
	}
	*encodedRoot = EncodeDirtyRoot(r)
}

// ClearDirtyTree：清空 dirty tree 并写回 NilIdx()。
func (op TreeOps) ClearDirtyTree(encodedRoot *int32) {
	if encodedRoot == nil {
		return
	}
	t := op.TreeFromEncodedRoot(*encodedRoot)
	t.FreeAll()
	op.SaveDirtyTree(encodedRoot, t)
}

// cloneDirtyEncodedRoot：仅当源为 dirtyRoot 时克隆 RBTree；
// 若源是 baseRoot（不可变）则直接原样返回（共享 base 安全）。
func cloneDirtyEncodedRoot(op TreeOps, srcEncoded int32) int32 {
	if !IsDirtyEncodedRoot(srcEncoded) {
		return srcEncoded
	}
	srcRoot := DecodeDirtyRoot(srcEncoded)
	if srcRoot < 0 {
		return NilIdx()
	}

	srcT := NewRichRangeTree(op.pool)
	srcT.SetRoot(srcRoot)

	dstRoot := int32(NilIdx())
	dstT := NewRichRangeTree(op.pool)
	dstT.SetRoot(dstRoot)

	srcT.ForeachAll(func(rr RichRange) bool {
		dstT.Insert(rr)
		return true
	})

	r := dstT.Root()
	if r < 0 {
		return NilIdx()
	}
	return EncodeDirtyRoot(r)
}

// ensureWritableSpan：保证 subIdx 指向“可写”的 span（映射上独占）。
// - 若 span 被多个 subIdx 共享：分配新 span，并把 subIdx 重映射过去。
// - 若旧 root 是 dirty 且共享：需要 clone dirty tree，避免影响其它 sub。
// - 若旧 root 是 base：base 不可变，共享安全，不需要 clone（后续由调用处决定何时 materialize 成 dirty）。
func (hp *HighPrecisionColumn) ensureWritableSpan(subIdx int, op TreeOps) (*int32, uint8) {
	if subIdx < 0 || subIdx >= SecondaryTileNum {
		return nil, 0
	}
	hp.ensureInit()

	// 首次创建覆盖：分配新 span
	if !hp.HasSpan(subIdx) {
		spanIdx := hp.allocSpan()
		hp.setHas(subIdx, true)
		hp.Same.Set(subIdx, spanIdx)
		return &hp.Spans[spanIdx], spanIdx
	}

	spanIdx := hp.Same.Get(subIdx)
	if int(spanIdx) < 0 || int(spanIdx) >= len(hp.Spans) {
		spanIdx2 := hp.allocSpan()
		hp.Same.Set(subIdx, spanIdx2)
		return &hp.Spans[spanIdx2], spanIdx2
	}

	// 独占：可写
	if hp.refCount(spanIdx) == 1 {
		return &hp.Spans[spanIdx], spanIdx
	}

	// 共享：COW（仅当 dirtyRoot 才需要 clone）
	oldEncoded := hp.Spans[spanIdx]
	newSpan := hp.allocSpan()

	if IsDirtyEncodedRoot(oldEncoded) {
		hp.Spans[newSpan] = cloneDirtyEncodedRoot(op, oldEncoded)
	} else {
		// baseRoot 或 nil：共享安全，直接拷贝
		hp.Spans[newSpan] = oldEncoded
	}
	hp.Same.Set(subIdx, newSpan)
	return &hp.Spans[newSpan], newSpan
}

// ensureWritableSpanOverwrite：subIdx 将被“直接写入一个全新 root”，不需要 clone 老树。
// 若当前 span 共享，会分配新 span 并仅重映射 subIdx。
func (hp *HighPrecisionColumn) ensureWritableSpanOverwrite(subIdx int) (*int32, uint8) {
	if subIdx < 0 || subIdx >= SecondaryTileNum {
		return nil, 0
	}
	hp.ensureInit()

	if !hp.HasSpan(subIdx) {
		spanIdx := hp.allocSpan()
		hp.setHas(subIdx, true)
		hp.Same.Set(subIdx, spanIdx)
		return &hp.Spans[spanIdx], spanIdx
	}

	spanIdx := hp.Same.Get(subIdx)
	if int(spanIdx) < 0 || int(spanIdx) >= len(hp.Spans) {
		spanIdx2 := hp.allocSpan()
		hp.Same.Set(subIdx, spanIdx2)
		return &hp.Spans[spanIdx2], spanIdx2
	}

	if hp.refCount(spanIdx) == 1 {
		return &hp.Spans[spanIdx], spanIdx
	}

	newSpan := hp.allocSpan()
	hp.Same.Set(subIdx, newSpan)
	return &hp.Spans[newSpan], newSpan
}
