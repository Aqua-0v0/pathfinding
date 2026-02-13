package zmap3base

import (
	"runtime"
	"sort"
)

// Env 地图数据
type Env struct {
	rect       Rect
	minX, minY uint16 // MinX, MinY 快取.

	gridW, gridH uint16

	grids []*GridRBData // len = gridW*gridH
}

func NewEnv(rect Rect) *Env {
	var env = &Env{
		rect: rect,
		minX: rect.Min.X, minY: rect.Min.Y,
		gridW: (rect.Width() + FastGridSetSize - 1) / FastGridSetSize,
		gridH: (rect.Height() + FastGridSetSize - 1) / FastGridSetSize,
	}
	env.grids = make([]*GridRBData, int(env.gridW*env.gridH))
	return env
}

// Destroy 销毁 Env 所有数据, 释放内存.
func (e *Env) Destroy() {
	defer runtime.GC()
	e.grids = nil
}

func (e *Env) Rect() Rect {
	return e.rect
}

func (e *Env) MinX() uint16 {
	return e.minX
}
func (e *Env) MaxX() uint16 {
	return e.rect.Max.X - 1
}
func (e *Env) MinY() uint16 {
	return e.minY
}
func (e *Env) MaxY() uint16 {
	return e.rect.Max.Y - 1
}

// MapWidth 获取地图宽度, 即根部 Env 的宽度.
func (e *Env) MapWidth() uint16 {
	return e.rect.Width()
}

// MapHeight 获取地图长度, 即根部 Env 的长度.
func (e *Env) MapHeight() uint16 {
	return e.rect.Height()
}

func (e *Env) String() string {
	return e.rect.String()
}

// routeLP：专门给“必须落在 LP tile”的逻辑用（比如写 Terrain/Climate）
func (e *Env) routeLP(p Point2d) (g *GridRBData, lp Point2d, cellIdx int, ok bool) {
	lp = p.LowPrecisionPoint()
	if !e.Validate2d(p) {
		return nil, Point2d{}, 0, false
	}
	g = e.gridOfPoint(lp)
	if g == nil {
		return nil, Point2d{}, 0, false
	}
	return g, lp, g.CellIdx(lp.X, lp.Y), true
}

// Validate2d 检查 zmap3types.Point2d 坐标范围是否有效.
func (e *Env) Validate2d(p Point2d) bool {
	return e.minX <= p.X && p.X < e.rect.Max.X && e.minY <= p.Y && p.Y < e.rect.Max.Y
}

// gridIdx = ((x-minX)/32) + ((y-minY)/32)*gridW
func (e *Env) gridIdxOf(x, y uint16) int {
	gx := int((x - e.minX) >> 5) // /32
	gy := int((y - e.minY) >> 5)
	return gx + gy*int(e.gridW)
}

func (e *Env) gridOfPoint(lp Point2d) *GridRBData {
	if lp.X < e.rect.Min.X || lp.X >= e.rect.Max.X || lp.Y < e.rect.Min.Y || lp.Y >= e.rect.Max.Y {
		return nil
	}
	i := e.gridIdxOf(lp.X, lp.Y)
	if i < 0 || i >= len(e.grids) {
		return nil
	}
	return e.grids[i]
}

type routeCtx struct {
	g         *GridRBData
	sourceP2d Point2d
	cellIdx   int

	// HP
	isHP   bool
	subIdx int
}

// route 路由 Point2d 到对应的 grid + cellIdx + HP 信息.
func (e *Env) route(p Point2d) (c routeCtx, ok bool) {
	if !e.Validate2d(p) {
		return routeCtx{}, false
	}

	// 2) 找 grid
	g := e.gridOfPoint(p)
	if g == nil {
		return routeCtx{}, false
	}

	// 3) cellIdx
	cellIdx := g.CellIdx(p.X, p.Y)

	// 4) HP 判定（只要 offset 非 0 就认为用户在请求 HP）
	isHP := (p.XOffset != 0 || p.YOffset != 0)
	subIdx := 0
	if isHP {
		si, ok2 := SubIdxFromPoint2d(p)
		if !ok2 {
			return routeCtx{}, false
		}
		subIdx = si
	}

	return routeCtx{
		g:         g,
		sourceP2d: p,
		cellIdx:   cellIdx,
		isHP:      isHP,
		subIdx:    subIdx,
	}, true
}

func cellHasAnyHP(d *RichRangeSetData) bool {
	return d != nil && d.HighPrecision != nil && d.HighPrecision.Has != 0
}

func cellHasBaseHP(d *RichRangeSetData) bool {
	if d == nil || d.HighPrecision == nil || d.HighPrecision.Has == 0 || len(d.HighPrecision.Spans) == 0 {
		return false
	}
	for i := 0; i < len(d.HighPrecision.Spans); i++ {
		if IsBaseEncodedRoot(d.HighPrecision.Spans[i]) {
			return true
		}
	}
	return false
}

func terrainFromBaseSlice(seg []RichRange) (RichRange, bool) {
	if len(seg) == 0 {
		return RichRange{}, false
	}
	// header 形式：seg[0].Begin==len(seg)，End=terrainEnd
	if len(seg) > 0 && int(seg[0].Begin) == len(seg) {
		hdr := seg[0]
		if hdr.End > 0 {
			rr := hdr
			rr.Range = Range{0, hdr.End}
			return rr, true
		}
	}
	// fallback：在 payload 里找 Begin==0 && MaterBase
	var best RichRange
	bestEnd := uint16(0)
	found := false
	for i := 0; i < len(seg); i++ {
		rr := seg[i]
		if rr.Begin != 0 || rr.End == 0 || rr.Range.Len() == 0 {
			continue
		}
		if (rr.Accessory.Texture & TextureMaterBase) == 0 {
			continue
		}
		if !found || rr.End > bestEnd {
			best = rr
			bestEnd = rr.End
			found = true
		}
	}
	return best, found
}

func terrainFromDirty(op TreeOps, encodedRoot int32) (RichRange, bool) {
	if !IsDirtyEncodedRoot(encodedRoot) {
		return RichRange{}, false
	}
	root := DecodeDirtyRoot(encodedRoot)
	if root < 0 {
		return RichRange{}, false
	}
	// 防止 root 指向空/错 pool
	if op.pool == nil || int(root) >= len(op.pool.nodes) {
		return RichRange{}, false
	}
	t := NewRichRangeTree(op.pool)
	t.SetRoot(root)

	// 在 [0,1) 里找 Begin==0 的 MaterBase 段
	q := Range{0, 1}
	var best RichRange
	bestEnd := uint16(0)
	found := false
	t.RangeQueryInOrder(q, func(rr RichRange) bool {
		if rr.Begin != 0 || rr.End == 0 || rr.Range.End-rr.Range.Begin == 0 {
			return true
		}
		if (rr.Accessory.Texture & TextureMaterBase) == 0 {
			return true
		}
		//if rr.Accessory.Texture&MaterBase == 0 {
		//	return true
		//}
		if !found || rr.End > bestEnd {
			best = rr
			bestEnd = rr.End
			found = true
		}
		return true
	})
	if found {
		return best, true
	}

	// fallback：任意 Begin==0 的最长段
	t.RangeQueryInOrder(q, func(rr RichRange) bool {
		if rr.Begin != 0 || rr.End == 0 || rr.Range.End-rr.Range.Begin == 0 {
			return true
		}
		if !found || rr.End > bestEnd {
			best = rr
			bestEnd = rr.End
			found = true
		}
		return true
	})
	return best, found
}

// terrainRR：从 cell 的 LP source 里复原 Terrain RichRange（Begin=0..terrainEnd）
func terrainRR(g *GridRBData, cellIdx int) (RichRange, bool) {
	d := g.CellByIdx(cellIdx)
	if d == nil {
		return RichRange{}, false
	}
	// base
	if IsBaseEncodedRoot(d.RootNode) {
		seg := g.base.GetSlice(DecodeBaseRoot(d.RootNode))
		return terrainFromBaseSlice(seg)
	}
	// dirty
	if IsDirtyEncodedRoot(d.RootNode) {
		return terrainFromDirty(g.Ops(), d.RootNode)
	}
	return RichRange{}, false
}

// CheckBaseHeightLoaded : 检查基础高度是否加载
func (e *Env) CheckBaseHeightLoaded(p Point2d) bool {
	rc, ok := e.route(p)
	if !ok {
		return false
	}
	d := rc.g.CellByIdx(rc.cellIdx)
	return d != nil
}

// GetIsHighPrecision 查询 Point2d 的位置是否是高精点
func (e *Env) GetIsHighPrecision(p Point2d) bool {
	rc, ok := e.route(p)
	if !ok {
		return false
	}
	return rc.isHP
}

func appendRangesFromSource(op TreeOps, root int32, base []RichRange, override bool, out []RichRange) []RichRange {
	if override {
		if root < 0 {
			return out
		}
		t := NewRichRangeTree(op.pool)
		t.SetRoot(root)
		t.ForeachAll(func(rr RichRange) bool {
			if rr.End > 0 && rr.Range.Len() > 0 {
				out = append(out, rr)
			}
			return true
		})
		return out
	}

	rangeQueryBaseSlice(base, MaxRange, func(rr RichRange) bool {
		if rr.End > 0 && rr.Range.Len() > 0 {
			out = append(out, rr)
		}
		return true
	})
	return out
}

// GetTerrainAndSpansByPoint returns terrain and blocking spans for one point.
// Spans are sorted by (End asc, Begin asc), matching map_data.GetInterval expectations.
func (e *Env) GetTerrainAndSpansByPoint(p Point2d) (terrain RichRange, spans []RichRange, ok bool) {
	rc, ok := e.route(p)
	if !ok {
		return RichRange{}, nil, false
	}

	g := rc.g
	d := g.CellByIdx(rc.cellIdx)
	if d == nil {
		return RichRange{}, nil, false
	}

	terrain, ok = terrainRR(g, rc.cellIdx)
	if !ok {
		return RichRange{}, nil, false
	}

	hasAnyHP := cellHasAnyHP(d)
	effIsHP := rc.isHP
	effSubIdx := rc.subIdx
	if effIsHP {
		if !hasAnyHP {
			effIsHP = false
			effSubIdx = 0
		}
	} else if hasAnyHP {
		effIsHP = true
		effSubIdx = 0
	}

	op := g.Ops()
	spans = make([]RichRange, 0, 16)

	lpRoot, lpBase, lpOverride := g.lpSource(rc.cellIdx)
	spans = appendRangesFromSource(op, lpRoot, lpBase, lpOverride, spans)

	if effIsHP {
		hpRoot, hpBase, hpOverride := g.hpSource(rc.cellIdx, effSubIdx)
		spans = appendRangesFromSource(op, hpRoot, hpBase, hpOverride, spans)
	}

	// Terrain is returned separately; remove exact duplicate entries from spans.
	w := 0
	for i := 0; i < len(spans); i++ {
		rr := spans[i]
		if rr.Range == terrain.Range && rr.Accessory == terrain.Accessory {
			continue
		}
		spans[w] = rr
		w++
	}
	spans = spans[:w]

	sort.Slice(spans, func(i, j int) bool {
		if spans[i].End == spans[j].End {
			return spans[i].Begin < spans[j].Begin
		}
		return spans[i].End < spans[j].End
	})

	return terrain, spans, true
}

// addRangePoint 添加 RangePoint 到 Env 中.
// - p 是 LP：写入 LP tree（整 tile 共享），影响该 tile 的所有查询（包括 HP 点，因为查询会 merge LP+HP）
// - p 是 HP：只写入该 subIdx 的 HP tree（增量覆盖），不影响其他 subIdx
func (e *Env) addRangePoint(p Point3d, accessory Accessory) bool {
	p2d := p.Point2d()

	rc, ok := e.route(p2d)
	if !ok {
		return false
	}

	rg := Range{p.H, p.RangeEnd}

	rr := RichRange{
		Range:     rg,
		Accessory: accessory,
	}

	if !rc.isHP {
		rootPtr := rc.g.ensureDirtyLP(rc.cellIdx)
		return rc.g.includeOnRoot(rootPtr, rr)
	}

	baseHP := rc.g.BaseHPOf(rc.cellIdx, rc.subIdx)
	rootPtr := rc.g.ensureDirtyHP(rc.cellIdx, rc.subIdx, baseHP)
	return rc.g.includeOnRoot(rootPtr, rr)
}

// removeRangePoint 从 EnvRB 中删除 RangePoint（overlay 区间）。
//   - 若输入是“同精度点”（GetByPoint2d 归一化后仍是同一个点）：只改该点对应的树（LP 或该 subIdx 的 HP）
//   - 若输入是 LP，但该 tile 存在 HP（GetByPoint2d 会把点归一化到默认 HP 点）：则视作“删除 LP 点但原数据是 HP”
//     => 对该 tile 的所有 HP subIdx 生效（对齐老逻辑的 LoopHighPrecisionPoint）。
//   - 若输入是 HP，但该 tile 没有 HP：对齐老逻辑，返回 false,false（不支持从低精中删除高精）。
func (e *Env) removeRangePoint(p Point3d, accessory Accessory) (isHeightPChange, succ bool) {
	p2d := p.Point2d()
	rc, ok := e.route(p2d)
	if !ok {
		return false, false
	}

	g := rc.g
	cellIdx := rc.cellIdx
	d := g.CellByIdx(cellIdx)
	if d == nil {
		return false, false
	}

	exc := Range{p.H, p.RangeEnd}

	cfg := accessory.Config
	changedLP := false
	changedHP := false

	if !rc.isHP {
		rootPtr := g.ensureDirtyLP(cellIdx)
		changedLP = g.excludeOnRoot(rootPtr, exc, cfg)

		// 若 tile 有 HP（base 或 dirty），按 old 语义：对“已有覆盖的 sub”逐个做删（不创建新 sub）
		if cellHasAnyHP(d) {
			hp := d.HighPrecision
			for sub := 0; sub < SecondaryTileNum; sub++ {
				if hp == nil || !hp.HasSpan(sub) {
					continue
				}
				baseHP := g.BaseHPOf(cellIdx, sub)
				hpRoot := g.ensureDirtyHP(cellIdx, sub, baseHP)
				if hpRoot != nil {
					if g.excludeOnRoot(hpRoot, exc, cfg) {
						changedHP = true
					}
				}
			}
		}
	} else {
		// HP 删除但 tile 没 HP：保持 old 语义
		if !cellHasAnyHP(d) {
			return false, false
		}
		if d.HighPrecision == nil || !d.HighPrecision.HasSpan(rc.subIdx) {
			return false, false
		}

		baseHP := g.BaseHPOf(cellIdx, rc.subIdx)
		hpRoot := g.ensureDirtyHP(cellIdx, rc.subIdx, baseHP)
		if hpRoot != nil {
			changedHP = g.excludeOnRoot(hpRoot, exc, cfg)
		}
	}

	// 关键：没删到任何东西 => 视为失败（old 基本也是这种效果）
	if !changedLP && !changedHP {
		return false, false
	}

	// isHeightPChange：只要 HP 有变化，就认为“高度点变化”
	return changedHP, true
}

// SkyNeighbour ：检查 Point3d 水平位置自顶向下是否存在符合条件的 RichRange.
// 语义对齐老版本：返回“最高的一个”满足 excludes 的区间（等价于从上往下扫到的第一个）。
func (e *Env) SkyNeighbour(p Point3d, excludes ...func(txt Texture, rng Range) bool) (snp SnapRichRange, ok bool) {
	p2d := p.Point2d()
	lp := p2d.LowPrecisionPoint()
	if !e.Validate2d(lp) {
		return SnapRichRange{}, false
	}

	rc, ok0 := e.route(p2d)
	if !ok0 {
		return SnapRichRange{}, false
	}
	g := rc.g
	d := g.CellByIdx(rc.cellIdx)
	if d == nil {
		return SnapRichRange{}, false
	}
	op := g.Ops()

	// 精度对齐 old GetByPoint2d
	hasAnyHP := cellHasAnyHP(d)
	effIsHP := rc.isHP
	effSubIdx := rc.subIdx
	if effIsHP {
		if !hasAnyHP {
			effIsHP = false
			effSubIdx = 0
		}
	} else {
		if hasAnyHP {
			effIsHP = true
			effSubIdx = 0
		}
	}

	accept := func(rr RichRange) bool {
		if rr.End == 0 || rr.Range.Len() == 0 {
			return false
		}
		for _, ex := range excludes {
			if ex(rr.Accessory.Texture, rr.Range) {
				return false
			}
		}
		return true
	}

	bestEnd := uint16(0)
	bestBegin := uint16(0)
	var bestRR RichRange
	bestSet := false

	tryUpdateBest := func(rr RichRange) {
		if !accept(rr) {
			return
		}
		e0, b0 := rr.End, rr.Begin
		if !bestSet || e0 > bestEnd || (e0 == bestEnd && b0 > bestBegin) {
			bestSet = true
			bestEnd, bestBegin = e0, b0
			bestRR = rr
		}
	}

	// 1) Terrain
	if ter, okTer := terrainRR(g, rc.cellIdx); okTer {
		tryUpdateBest(ter)
	}

	// 2) LP
	{
		root, base, override := g.lpSource(rc.cellIdx)
		if override {
			if root >= 0 {
				t := NewRichRangeTree(op.pool)
				t.SetRoot(root)
				be, bb, rr := t.FindMaxEndLEFull(MaxRangeEnd, bestEnd, bestBegin, bestRR, accept)
				if !bestSet && rr.End != 0 && rr.Range.Len() != 0 {
					bestSet = true
				}
				bestEnd, bestBegin, bestRR = be, bb, rr
			}
		} else {
			rangeQueryBaseSlice(base, MaxRange, func(rr RichRange) bool {
				tryUpdateBest(rr)
				return true
			})
		}
	}

	// 3) HP：effIsHP 才参与
	if effIsHP {
		root, base, override := g.hpSource(rc.cellIdx, effSubIdx)
		if override {
			if root >= 0 {
				t := NewRichRangeTree(op.pool)
				t.SetRoot(root)
				be, bb, rr := t.FindMaxEndLEFull(MaxRangeEnd, bestEnd, bestBegin, bestRR, accept)
				if !bestSet && rr.End != 0 && rr.Range.Len() != 0 {
					bestSet = true
				}
				bestEnd, bestBegin, bestRR = be, bb, rr
			}
		} else {
			rangeQueryBaseSlice(base, MaxRange, func(rr RichRange) bool {
				tryUpdateBest(rr)
				return true
			})
		}
	}

	if !bestSet || bestRR.Range.Len() == 0 || bestRR.End == 0 {
		return SnapRichRange{}, false
	}

	return SnapRichRange{
		Range:   bestRR.Range,
		Texture: bestRR.Accessory.Texture,
	}, true
}

// ApplyRichOperationsExt 应用一组 RichOperation.
// 新结构版：
// 1) 先 addRangePoint
// 2) 再 removeRangePoint
// 3) 对 remove 中 isHeightPChange=true 的 LP cell：若该 cell 的 16 个 HP sub 的最终视图完全一致，则折叠回 LP：
//   - LP 变成 dirty 并重建为该最终视图（去掉 Terrain）
//   - 清空 HP dirty；若存在 baseHP，则建立“统一 override-empty”阻断 baseHP 回落
func (e *Env) ApplyRichOperationsExt(addRangePoint, removeRangePoint []Point3d, accessory Accessory) (ok bool) {
	//打印堆栈
	if len(addRangePoint) == 0 && len(removeRangePoint) == 0 {
		return true
	}
	ok = true

	// 1) add
	for _, p := range addRangePoint {
		succ := e.addRangePoint(p, accessory)
		if !succ {
			ok = false
		}
	}

	// 2) remove：收集发生“高度点变化”的 LP 点（去重）
	lpChanged := make(map[Point2d]struct{}, len(removeRangePoint))
	for _, p := range removeRangePoint {
		isHeightPChange, succ := e.removeRangePoint(p, accessory)
		if !succ {
			ok = false
			continue
		}
		if isHeightPChange {
			lpChanged[p.Point2d().LowPrecisionPoint()] = struct{}{}
		}
	}

	// 3) 尝试折叠回 LP
	for lp := range lpChanged {
		e.tryFoldHPToLPIfUniform(lp)
	}

	return ok
}

// tryFoldHPToLPIfUniform：若 lp 所在 cell 的 16 个 HP sub 的最终 RichRanges 全相同，则折叠回 LP。w
func (e *Env) tryFoldHPToLPIfUniform(lp Point2d) {
	if !e.Validate2d(lp) {
		return
	}

	// 取 grid/cell
	g, _, cellIdx, ok := e.routeLP(lp)
	if !ok || g == nil {
		return
	}

	d := g.CellByIdx(cellIdx)
	if d == nil || !cellHasAnyHP(d) {
		return
	}

	// 如果当前仍存在 baseHP（span root 仍是 base），不 fold（避免丢 base 语义）
	if cellHasBaseHP(d) {
		return
	}

	// 16 个 sub 的最终 rr 列表比较
	var (
		first    []RichRange
		hasFirst bool
		canFold  = true
	)

	lp.LoopHighPrecisionPointExt(func(hpP Point2d) bool {
		rrs := collectMergedRRsSorted(e, hpP)
		if !hasFirst {
			first = rrs
			hasFirst = true
			return true
		}
		if !equalRRs(first, rrs) {
			canFold = false
			return false
		}
		return true
	})

	if !canFold || !hasFirst {
		return
	}

	e.foldHPIntoLPAndDropHP(g, cellIdx)
}

func (e *Env) foldHPIntoLPAndDropHP(g *GridRBData, cellIdx int) {
	d := g.CellByIdx(cellIdx)
	if d == nil || d.HighPrecision == nil || d.HighPrecision.Has == 0 {
		return
	}
	op := g.Ops()

	// 1) 先确保 LP dirty（把 baseLP 物化进来，确保后续能屏蔽 base）
	lpRootPtr := g.ensureDirtyLP(cellIdx)
	if lpRootPtr == nil {
		return
	}
	lpTree := op.TreeFromEncodedRoot(*lpRootPtr)

	// 2) 从代表 sub（默认 sub=0，即(1,1)）把 HP overlays 搬进 LP
	const repSub = 0
	root, base, override := g.hpSource(cellIdx, repSub)
	if override {
		if root >= 0 {
			t := NewRichRangeTree(op.pool)
			t.SetRoot(root)
			t.ForeachAll(func(rr RichRange) bool {
				if rr.End == 0 || rr.Range.Len() == 0 {
					return true
				}
				lpTree.Insert(rr)
				return true
			})
		}
	} else {
		// base slice（理论上这里不应出现，因为我们已禁止 baseHP fold；但防御一下）
		rangeQueryBaseSlice(base, MaxRange, func(rr RichRange) bool {
			if rr.End == 0 || rr.Range.Len() == 0 {
				return true
			}
			lpTree.Insert(rr)
			return true
		})
	}

	op.SaveDirtyTree(lpRootPtr, lpTree)

	// 3) 清空 HP dirty 树并 drop HP
	hp := d.HighPrecision
	hp.ensureInit()
	for i := 0; i < len(hp.Spans); i++ {
		// 只清 dirty；base 理论上已被挡住（上面已 return），这里仍做防御：直接置空
		if IsDirtyEncodedRoot(hp.Spans[i]) {
			op.ClearDirtyTree(&hp.Spans[i])
		}
		hp.Spans[i] = NilIdx()
	}
	d.HighPrecision = nil
}

// collectMergedRRsSorted：取某点的“最终合并视图”(OR snapshot)并排序用于比较
func collectMergedRRsSorted(e *Env, p Point2d) []RichRange {
	out := make([]RichRange, 0, 16)

	// 省略：
	// 遍历该点的所有range 并 append 到out
	sort.Slice(out, func(i, j int) bool {
		return cmpRichRange(out[i], out[j]) < 0
	})
	return out
}

func equalRRs(a, b []RichRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Range != b[i].Range {
			return false
		}
		if a[i].Accessory != b[i].Accessory {
			return false
		}
	}
	return true
}

// orCoverZeroIgnoredTex：回补 “覆盖高度 0” 的、且满足 isIgnored 的 RR 的纹理。
// 用于 gap.Begin==0 时的 UseIgnoreTexture 语义对齐：旧结构的 stackTxt 能在起点就带出 ignored floor rr。
func (e *Env) orCoverZeroIgnoredTex(p2d Point2d, isIgnored func(rr RichRange) bool) (out Texture) {
	rc, ok := e.route(p2d)
	if !ok {
		return 0
	}
	g := rc.g
	cellIdx := rc.cellIdx
	d := g.CellByIdx(cellIdx)
	if d == nil {
		return 0
	}
	op := g.Ops()

	// 精度对齐 old GetByPoint2d：HP 不存在则降级；LP 但 tile 有 HP 则默认 HP(1,1)
	hasAnyHP := cellHasAnyHP(d)
	effIsHP := rc.isHP
	effSubIdx := rc.subIdx
	if effIsHP {
		if !hasAnyHP {
			effIsHP = false
			effSubIdx = 0
		}
	} else {
		if hasAnyHP {
			effIsHP = true
			effSubIdx = 0
		}
	}

	// 只关心覆盖 0 的 rr，因此查询 [0,1)
	q := Range{0, 1}

	visit := func(rr RichRange) {
		if rr.End == 0 || rr.Range.End-rr.Range.Begin == 0 {
			return
		}
		// 必须覆盖 0：Begin==0 && End>0
		if rr.Begin != 0 || rr.End <= 0 {
			return
		}
		// 再做一次 overlap（防御）
		if !rr.Range.Overlap(q) {
			return
		}
		if isIgnored != nil && !isIgnored(rr) {
			return
		}
		out |= rr.Accessory.Texture
	}

	// 1) Terrain（从 cell 的 LP source 复原）
	if ter, okTer := terrainRR(g, cellIdx); okTer {
		visit(ter)
	}

	// 2) LP overlays
	{
		root, base, override := g.lpSource(cellIdx)
		if override {
			if root >= 0 {
				t := NewRichRangeTree(op.pool)
				t.SetRoot(root)
				t.RangeQueryInOrder(q, func(rr RichRange) bool {
					visit(rr)
					return true
				})
			}
		} else {
			rangeQueryBaseSlice(base, q, func(rr RichRange) bool {
				visit(rr)
				return true
			})
		}
	}

	// 3) HP overlays（仅 effIsHP 才叠加）
	if effIsHP {
		root, base, override := g.hpSource(cellIdx, effSubIdx)
		if override {
			if root >= 0 {
				t := NewRichRangeTree(op.pool)
				t.SetRoot(root)
				t.RangeQueryInOrder(q, func(rr RichRange) bool {
					visit(rr)
					return true
				})
			}
		} else {
			rangeQueryBaseSlice(base, q, func(rr RichRange) bool {
				visit(rr)
				return true
			})
		}
	}

	return out
}
