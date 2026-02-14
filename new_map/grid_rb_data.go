package zmap3base

// GridRBData：一个 32x32 grid 的数据块
type GridRBData struct {
	// 该 grid 左下角（对齐 32 的全局坐标）
	baseX, baseY uint16

	// 32*32 cells：每个 cell 的“可变部分”
	// - Dirty LP root  LP:低精度点 1x1
	// - Dirty HP column（按需创建） HP:高精度点 0.25x0.25
	cells [FastGridCellNum]RichRangeSetData

	// Base：只读初始化数据（数组存储）
	base BaseStore

	dirtyPool *NodePool
	dirtyOps  TreeOps
}

// NewGridRBData ：创建一个 grid 数据块
func NewGridRBData(baseX, baseY uint16, dirtyCapHint int) *GridRBData {
	g := GetGridRBDataFromPool()
	g.baseX = baseX
	g.baseY = baseY
	// cells 默认 RootNode=NilIdx，HighPrecision=nil，Terrain End=0 都是空
	for i := range g.cells {
		g.cells[i].RootNode = 0
	}
	if dirtyCapHint < 0 {
		dirtyCapHint = 0
	}
	g.dirtyPool = NewNodePool(dirtyCapHint)
	g.dirtyOps.pool = g.dirtyPool
	return g
}

func (g *GridRBData) InitBase(base BaseStore) {
	g.base = base
}

func (g *GridRBData) BaseX() uint16 { return g.baseX }
func (g *GridRBData) BaseY() uint16 { return g.baseY }
func (g *GridRBData) Ops() TreeOps  { return g.dirtyOps }

func (g *GridRBData) CellIdx(x, y uint16) int {
	// 这里假设传入 x,y 一定属于该 grid（由 Env 路由保证）
	dx := int(x - g.baseX) // 0..31
	dy := int(y - g.baseY) // 0..31
	return dx + (dy << 5)  // dy*32 + dx
}

func (g *GridRBData) CellByIdx(cellIdx int) *RichRangeSetData {
	if cellIdx < 0 || cellIdx >= FastGridCellNum {
		return nil
	}
	return &g.cells[cellIdx]
}

// ======================= base getters =======================

// BaseLPOf：仅当当前 cell RootNode 指向 base 段时返回 base slice（包含 header）；否则返回 nil。
func (g *GridRBData) BaseLPOf(cellIdx int) []RichRange {
	d := g.CellByIdx(cellIdx)
	if d == nil {
		return nil
	}
	if !IsBaseEncodedRoot(d.RootNode) {
		return nil
	}
	return g.base.GetSlice(DecodeBaseRoot(d.RootNode))
}

// BaseHPOf：仅当该 subIdx 的 span root 指向 base 段时返回 base slice（包含 header）；否则返回 nil。
func (g *GridRBData) BaseHPOf(cellIdx int, subIdx int) []RichRange {
	d := g.CellByIdx(cellIdx)
	if d == nil || d.HighPrecision == nil {
		return nil
	}
	if subIdx < 0 || subIdx >= SecondaryTileNum {
		return nil
	}
	hp := d.HighPrecision
	if !hp.HasSpan(subIdx) {
		return nil
	}
	spanIdx := int(hp.Same.Get(subIdx))
	if spanIdx < 0 || spanIdx >= len(hp.Spans) {
		return nil
	}
	r := hp.Spans[spanIdx]
	if !IsBaseEncodedRoot(r) {
		return nil
	}
	return g.base.GetSlice(DecodeBaseRoot(r))
}

func basePayloadIsEmpty(seg []RichRange) bool {
	// seg 可能是 nil/空
	if len(seg) == 0 {
		return true
	}
	// 若带 header（Begin==len(seg)），payload 是 seg[1:]
	if len(seg) > 0 && int(seg[0].Begin) == len(seg) {
		return len(seg) <= 1
	}
	// 不带 header：认为全是 payload
	return len(seg) == 0
}

// ======================= materialize base -> dirty =======================

// materializeLPBaseToDirty：把 LP base 变成 dirty RBTree（并返回 dirty encoded root 或 NilIdx）。
// 约定：LP 段 header：seg[0].Begin==len(seg)，seg[0].End=terrainEnd，且 seg[0] 的 Accessory 是 terrain 的 Accessory。
// 物化时要把 terrain 复原成 Begin=0 的 rr，再插入 payload(seg[1:])。
func (g *GridRBData) materializeLPBaseToDirty(baseRoot int32) int32 {
	seg := g.base.GetSlice(baseRoot)
	if len(seg) == 0 {
		return NilIdx()
	}

	t := NewRichRangeTree(g.dirtyPool)
	t.SetRoot(NilIdx())

	if len(seg) > 0 && int(seg[0].Begin) == len(seg) {
		// header -> terrain
		hdr := seg[0]
		terrain := hdr
		terrain.Range = Range{0, hdr.End}
		if terrain.Range.End-terrain.Range.Begin > 0 {
			t.Insert(terrain)
		}
		for i := 1; i < len(seg); i++ {
			t.Insert(seg[i])
		}
	} else {
		// defensive：无 header 就全量插入
		for i := 0; i < len(seg); i++ {
			t.Insert(seg[i])
		}
	}

	if t.Root() < 0 {
		return NilIdx()
	}
	return EncodeDirtyRoot(t.Root())
}

// materializeHPBaseToDirty：把 HP base 变成 dirty RBTree（返回 dirty encoded root 或 NilIdx）。
// 约定：HP 段 header：seg[0].Begin==len(seg) 且 seg[0].End==0；payload=seg[1:].
func (g *GridRBData) materializeHPBaseToDirty(baseRoot int32, segOpt []RichRange) int32 {
	seg := segOpt
	if seg == nil {
		seg = g.base.GetSlice(baseRoot)
	}
	if len(seg) == 0 {
		return NilIdx()
	}

	t := NewRichRangeTree(g.dirtyPool)
	t.SetRoot(NilIdx())

	start := 0
	if len(seg) > 0 && int(seg[0].Begin) == len(seg) {
		start = 1
	}
	for i := start; i < len(seg); i++ {
		t.Insert(seg[i])
	}

	if t.Root() < 0 {
		return NilIdx()
	}
	return EncodeDirtyRoot(t.Root())
}

// ======================= ensure dirty roots =======================

// ensureDirtyLP：确保 cellIdx 的 RootNode 已经是 dirtyRoot 或 NilIdx（用于后续修改）。
// - 若原本是 baseRoot：materialize base -> dirty，并写回 RootNode。
// - 若原本 nil/0：保持 NilIdx（空 dirty），后续 Insert 会变为 dirtyRoot。
func (g *GridRBData) ensureDirtyLP(cellIdx int) *int32 {
	d := g.CellByIdx(cellIdx)
	if d == nil {
		return nil
	}

	r := d.RootNode
	if IsDirtyEncodedRoot(r) || IsNilEncodedRoot(r) {
		if r == 0 {
			d.RootNode = NilIdx()
		}
		return &d.RootNode
	}

	// base -> dirty
	d.RootNode = g.materializeLPBaseToDirty(DecodeBaseRoot(r))
	return &d.RootNode
}

// ensureDirtyHP：确保该 subIdx 的 span root 已经可写且为 dirtyRoot 或 NilIdx。
// baseHP 可由调用方传入（包含 header），避免重复 GetSlice；传 nil 则内部按需拉取。
func (g *GridRBData) ensureDirtyHP(cellIdx int, subIdx int, baseHP []RichRange) *int32 {
	if cellIdx < 0 || cellIdx >= FastGridCellNum {
		return nil
	}
	if subIdx < 0 || subIdx >= SecondaryTileNum {
		return nil
	}

	d := &g.cells[cellIdx]
	if d.HighPrecision == nil {
		d.HighPrecision = new(HighPrecisionColumn)
	}
	hp := d.HighPrecision

	rootPtr, _ := hp.ensureWritableSpan(subIdx, g.dirtyOps)
	if rootPtr == nil {
		return nil
	}

	// 已经是 dirty 或 override-empty
	if IsDirtyEncodedRoot(*rootPtr) || IsNilEncodedRoot(*rootPtr) {
		if *rootPtr == 0 {
			*rootPtr = NilIdx()
		}
		return rootPtr
	}

	// base -> dirty materialize
	if IsBaseEncodedRoot(*rootPtr) {
		*rootPtr = g.materializeHPBaseToDirty(DecodeBaseRoot(*rootPtr), baseHP)
		return rootPtr
	}

	// 兜底：视为 override-empty
	*rootPtr = NilIdx()
	return rootPtr
}

// ======================= include/exclude on dirty tree =======================

// includeOnRoot：在 rootPtr 指向的“dirty tree（encoded）”上执行 include（单树近似 + touch 合并）。
// 约定：调用前必须 ensureDirtyLP/ensureDirtyHP，使 base 已物化为 dirty（否则会丢 base）。
func (g *GridRBData) includeOnRoot(rootPtr *int32, rr RichRange) bool {
	if rootPtr == nil {
		return false
	}
	if !rr.Valid() {
		return false
	}

	// old: clamp 到 MaxRange；Len==0 视为成功
	rr.Range = MaxRange.Intersect(rr.Range)
	if rr.Range.Len() == 0 {
		return true
	}

	t := g.dirtyOps.TreeFromEncodedRoot(*rootPtr)
	acc := rr.Accessory

	// 在单树里尽量对齐 old.Union：只合并相邻（touch）且 Accessory 全等的段
	mergedB, mergedE := rr.Begin, rr.End

	findTouchLeft := func(b uint16) (hit RichRange, ok bool) {
		if b == 0 {
			return RichRange{}, false
		}
		q := WrapRange(b-1, b) // [b-1, b)
		bestBegin := uint16(0)
		t.RangeQuery(q, func(x RichRange) bool {
			if x.Accessory != acc {
				return true
			}
			if x.End == b {
				// 多个候选时取 Begin 最大的（最贴近 b），更稳定
				if !ok || x.Begin > bestBegin {
					bestBegin = x.Begin
					hit = x
					ok = true
				}
			}
			return true
		})
		return hit, ok
	}

	findTouchRight := func(e uint16) (hit RichRange, ok bool) {
		if e >= MaxRangeEnd {
			return RichRange{}, false
		}
		q := WrapRange(e, e+1) // [e, e+1)
		bestEnd := uint16(0)

		t.RangeQuery(q, func(x RichRange) bool {
			if x.Accessory != acc {
				return true
			}
			if x.Begin == e {
				// 多个候选时取 End 最小的（最贴近 e），更稳定
				if !ok || x.End < bestEnd {
					bestEnd = x.End
					hit = x
					ok = true
				}
			}
			return true
		})
		return hit, ok
	}

	// 反复吸收左右 touch 邻居（链式合并）
	for {
		changed := false

		if lh, ok := findTouchLeft(mergedB); ok {
			t.DeleteExact(lh)
			mergedB = lh.Begin
			changed = true
		}
		if rh, ok := findTouchRight(mergedE); ok {
			t.DeleteExact(rh)
			mergedE = rh.End
			changed = true
		}

		if !changed {
			break
		}
	}

	rr.Range = WrapRange(mergedB, mergedE)
	t.Insert(rr)
	g.dirtyOps.SaveDirtyTree(rootPtr, t)
	return true
}

// excludeOnRoot：在 rootPtr 指向的“dirty tree（encoded）”上执行 exclude。
// - exc.Len()==0 => true
// - config==0：挖空所有命中段；只要原树非空就返回 true
// - config!=0：必须找到“Config==config 且 exc 被 rr 完全包含”的 rr 才成功
func (g *GridRBData) excludeOnRoot(rootPtr *int32, exc Range, config uint32) bool {
	if rootPtr == nil {
		return false
	}

	// old: clamp；Len==0 视为成功
	exc = MaxRange.Intersect(exc)
	if exc.Len() == 0 {
		return true
	}

	t := g.dirtyOps.TreeFromEncodedRoot(*rootPtr)
	if t.Root() < 0 {
		// old: 空集合 Exclude => false（config==0 也一样，raw 为空直接 false）
		return false
	}

	// config==0：挖空所有相交段（不要求包含）
	if config == 0 {

		// 收集命中段（不能边遍历边改树）
		var (
			small  [16]RichRange
			nSmall int
			list   []RichRange
		)
		t.RangeQueryInOrder(exc, func(hit RichRange) bool {
			if (hit.Accessory.Texture&TextureMaterBase) != 0 && hit.Begin == 0 {
				return true // 不允许挖 terrain
			}

			if nSmall < len(small) {
				small[nSmall] = hit
				nSmall++
			} else {
				if list == nil {
					list = make([]RichRange, 0, 32)
					list = append(list, small[:nSmall]...)
				}
				list = append(list, hit)
			}
			return true
		})

		// old 语义：raw 非空就算成功
		if list == nil && nSmall == 0 {
			return true
		}

		applyHit := func(hit RichRange) {
			// hit 与 exc 相交，挖空后最多两段残片
			itv := hit.Range.Intersect(exc)
			if itv.Len() == 0 {
				// 理论上 RangeQueryInOrder 命中的不会发生
				return
			}
			t.DeleteExact(hit)

			if hit.Begin < itv.Begin {
				left := hit
				left.Range = WrapRange(hit.Begin, itv.Begin)
				if left.Range.Len() > 0 {
					t.Insert(left)
				}
			}
			if itv.End < hit.End {
				right := hit
				right.Range = WrapRange(itv.End, hit.End)
				if right.Range.Len() > 0 {
					t.Insert(right)
				}
			}
		}

		if list != nil {
			for i := 0; i < len(list); i++ {
				applyHit(list[i])
			}
		} else {
			for i := 0; i < nSmall; i++ {
				applyHit(small[i])
			}
		}

		g.dirtyOps.SaveDirtyTree(rootPtr, t)
		return true
	}

	// config!=0：严格包含删除（对齐 old.RichRanges.Except）
	var (
		best    RichRange
		found   bool
		bestLen uint16
	)

	t.RangeQueryInOrder(exc, func(hit RichRange) bool {
		if (hit.Accessory.Texture&TextureMaterBase) != 0 && hit.Begin == 0 {
			return true // 不允许挖 terrain
		}
		if hit.Accessory.Config != config {
			return true
		}
		// 必须 exc 完全包含于 hit
		if !(hit.Begin <= exc.Begin && hit.End >= exc.End) {
			return true
		}

		// 选择策略：
		// 1) 优先 exact match（最符合直觉，也更接近 old 里“某个 channel 恰好有该段”）
		if hit.Range == exc {
			best = hit
			found = true
			return false
		}
		// 2) 否则取“包含 exc 的最短段”（避免误删更大的段）
		l := hit.Range.Len()
		if !found || l < bestLen {
			best = hit
			bestLen = l
			found = true
		}
		return true
	})

	if !found {
		return false
	}

	t.DeleteExact(best)

	// 回插残片（最多两段）
	if best.Begin < exc.Begin {
		left := best
		left.Range = WrapRange(best.Begin, exc.Begin)
		if left.Range.Len() > 0 {
			t.Insert(left)
		}
	}
	if exc.End < best.End {
		right := best
		right.Range = WrapRange(exc.End, best.End)
		if right.Range.Len() > 0 {
			t.Insert(right)
		}
	}

	g.dirtyOps.SaveDirtyTree(rootPtr, t)
	return true
}

// ======================= query sources =======================

// lpSource：给 LoopRichIntervals 供源选择
func (g *GridRBData) lpSource(cellIdx int) (root int32, base []RichRange, override bool) {
	d := g.CellByIdx(cellIdx)
	if d == nil {
		// route/cell 失败，上层一般当 interrupted
		return NilIdx(), nil, true
	}

	r := d.RootNode
	if r == NilIdx() {
		return NilIdx(), g.BaseLPOf(cellIdx), false
	}
	if r == 0 {
		return NilIdx(), nil, true
	}

	if IsDirtyEncodedRoot(r) {
		return DecodeDirtyRoot(r), nil, true
	}
	if IsBaseEncodedRoot(r) {
		// base 模式：返回 base slice（按 cellIdx 取即可）
		return NilIdx(), g.BaseLPOf(cellIdx), false
	}

	// 兜底：未知值不要屏蔽 base
	return NilIdx(), g.BaseLPOf(cellIdx), false
}

// hpSource：给 LoopRichIntervals 供源选择
func (g *GridRBData) hpSource(cellIdx int, subIdx int) (root int32, base []RichRange, override bool) {
	// base slice：就算没有 dirtyHP，也可能有 baseHP
	base = g.BaseHPOf(cellIdx, subIdx)

	d := g.CellByIdx(cellIdx)
	if d == nil || d.HighPrecision == nil || !d.HighPrecision.HasSpan(subIdx) {
		// 没有 dirtyHP span：直接用 base（base 可能为空）
		return NilIdx(), base, false
	}

	encodedRootPtr := d.HighPrecision.RootPtrOf(subIdx)
	if encodedRootPtr == nil {
		return NilIdx(), base, false
	}

	r := *encodedRootPtr
	if r == NilIdx() {
		// no dirty => 回退 base（关键）
		return NilIdx(), base, false
	}
	if r == 0 {
		// override-empty：强制屏蔽 base
		return NilIdx(), nil, true
	}

	if IsDirtyEncodedRoot(r) {
		return DecodeDirtyRoot(r), nil, true
	}
	if IsBaseEncodedRoot(r) {
		// base 模式：返回 base slice
		return NilIdx(), base, false
	}

	// 兜底：未知值不要屏蔽 base
	return NilIdx(), base, false
}

func baseSegSplit(rrs []RichRange) (hdr RichRange, payload []RichRange, hasHdr bool) {
	if len(rrs) == 0 {
		return RichRange{}, nil, false
	}
	// 只要 Begin==len(seg) 就是 header（len==1 也允许：仅 header 无 payload）
	if int(rrs[0].Begin) == len(rrs) {
		return rrs[0], rrs[1:], true
	}
	return RichRange{}, rrs, false
}

// baseSegStripHeader：只取 payload（给 rrSliceSrc / rrSrc.InitSlice 用）
func baseSegStripHeader(seg []RichRange) []RichRange {
	_, payload, _ := baseSegSplit(seg)
	return payload
}

// ---------------------- base cursor packing ----------------------
// base 游标需要同时携带：baseRootIdx + payload position
// - 游标必须 >0（LoopRichIntervals 用 idx==0 判结束）
// - 游标必须 < BaseThreshold（让 IsBaseEncodedRoot 判定为 base）
const basePosBits = 9
const basePosMask = (1 << basePosBits) - 1

func packBaseCursor(baseRootIdx int32, pos int32) int32 {
	// pos+1 避免 0
	return (baseRootIdx << basePosBits) | (pos + 1)
}

func unpackBaseCursor(cur int32) (baseRootIdx int32, pos int32) {
	baseRootIdx = cur >> basePosBits
	pos = (cur & basePosMask) - 1
	return
}

func isPackedBaseCursor(cur int32) bool {
	if cur <= 0 || cur >= BaseThreshold {
		return false
	}
	r, p := unpackBaseCursor(cur)
	return r >= 0 && p >= 0 // <- 关键：r 从 >0 改为 >=0
}

// GetRoots：返回 cellIdx 对应的 LP root 和 HP root（如果有的话）。
// 约定：调用前必须保证 p2d 在该 grid 内（由 Env 路由保证）。
// 如果不dirty ，则返回base root,如果 dirty 则返回 dirty root（可能是 NilIdx 或 override-empty）。
func (g *GridRBData) GetRoots(p2d Point2d) (lRoot int32, hRoot int32) {
	cellIdx := g.CellIdx(p2d.X, p2d.Y)
	d := g.CellByIdx(cellIdx)
	if d == nil {
		return NilIdx(), NilIdx()
	}

	// LP：直接返回 encoded root（base/dirty 都在这里）
	lRoot = d.RootNode

	// HP：拿 span 的 encoded root
	subIdx, _ := SubIdxFromPoint2d(p2d)
	if d.HighPrecision == nil || !d.HighPrecision.HasSpan(subIdx) {
		return lRoot, NilIdx()
	}
	encodedRootPtr := d.HighPrecision.RootPtrOf(subIdx)
	if encodedRootPtr == nil || *encodedRootPtr == 0 {
		return lRoot, NilIdx()
	}
	hRoot = *encodedRootPtr
	return
}

// DirtyFirstInOrderNode returns the left-most node index of a dirty encoded root.
// If encodedRoot is not dirty or invalid, it returns NilIdx().
func (g *GridRBData) DirtyFirstInOrderNode(encodedRoot int32) int32 {
	if !IsDirtyEncodedRoot(encodedRoot) || g.dirtyOps.pool == nil {
		return NilIdx()
	}
	root := DecodeDirtyRoot(encodedRoot)
	if root < 0 || int(root) >= len(g.dirtyOps.pool.nodes) {
		return NilIdx()
	}
	nodes := g.dirtyOps.pool.nodes
	cur := root
	for cur != NilIdx() {
		l := nodes[cur].left
		if l == NilIdx() {
			break
		}
		cur = l
	}
	return cur
}

// DirtyNextInOrderNode returns the in-order successor of nodeIdx in dirty tree.
// It returns NilIdx() if there is no successor.
func (g *GridRBData) DirtyNextInOrderNode(nodeIdx int32) int32 {
	return g.inorderSuccessor(nodeIdx)
}

// DirtyNodeRichRange returns the RichRange stored at nodeIdx of dirty tree.
func (g *GridRBData) DirtyNodeRichRange(nodeIdx int32) (rr RichRange, ok bool) {
	return g.nodeRichRange(nodeIdx)
}

// overlap test: rr 与查询范围 rrs.Range 是否有交集
func overlapRichRange(rr RichRange, q Range) bool {
	return rr.End > q.Begin && rr.Begin < q.End
}

// inorderSuccessor：找到 nodeIdx 在 dirty tree 中的中序后继节点索引
func (g *GridRBData) inorderSuccessor(nodeIdx int32) int32 {
	if nodeIdx < 0 || g.dirtyOps.pool == nil {
		return NilIdx()
	}
	nodes := g.dirtyOps.pool.nodes
	if int(nodeIdx) >= len(nodes) {
		return NilIdx()
	}

	n := &nodes[nodeIdx]

	// 1) 有右子树：successor = 右子树的最左
	if n.right != NilIdx() {
		x := n.right
		for x != NilIdx() {
			l := nodes[x].left
			if l == NilIdx() {
				break
			}
			x = l
		}
		return x
	}

	x := nodeIdx
	p := n.parent
	for p != NilIdx() && nodes[p].right == x {
		x = p
		p = nodes[p].parent
	}
	return p
}

func (g *GridRBData) nodeRichRange(nodeIdx int32) (rr RichRange, ok bool) {
	if g.dirtyOps.pool == nil {
		return RichRange{}, false
	}
	if nodeIdx < 0 || int(nodeIdx) >= len(g.dirtyOps.pool.nodes) {
		return RichRange{}, false
	}
	rr = g.dirtyOps.pool.nodes[nodeIdx].Range
	return rr, true
}
