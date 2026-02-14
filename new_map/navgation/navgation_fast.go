package navgation

import (
	"sync"

	zmap3base "pathfinding/new_map"
)

var fastRichRangeSlicePool = sync.Pool{
	New: func() any {
		buf := make([]zmap3base.RichRange, 0, 96)
		return &buf
	},
}

// GetIntervalFast is a lower-overhead variant of GetInterval.
// It reads LP/HP sources directly and avoids the generic collect helper.
func GetIntervalFast(
	env *zmap3base.Env,
	p2d zmap3base.Point2d,
	curY int32,
	ignoreTexture, forbiddenTexture uint32,
	height, upLimit, downLimit int32,
) (result zmap3base.SnapRichRange, ok bool) {
	if env == nil {
		return
	}
	rc, ok := env.Route(p2d)
	if !ok {
		return
	}
	return GetIntervalFastRouted(rc, curY, ignoreTexture, forbiddenTexture, height, upLimit, downLimit)
}

// GetIntervalFastRouted reuses a precomputed route context to save one route call.
func GetIntervalFastRouted(
	rc zmap3base.RouteCtx,
	curY int32,
	ignoreTexture, forbiddenTexture uint32,
	height, upLimit, downLimit int32,
) (result zmap3base.SnapRichRange, ok bool) {
	g := rc.G
	if g == nil {
		return
	}
	d := g.CellByIdx(rc.CellIdx)
	if d == nil {
		return
	}

	terrain, ok := resolveTerrainFast(g, d, rc.CellIdx)
	if !ok {
		return
	}

	hasAnyHP := d.HighPrecision != nil && d.HighPrecision.Has != 0
	effIsHP := rc.IsHP
	effSubIdx := rc.SubIdx
	if effIsHP {
		if !hasAnyHP {
			effIsHP = false
			effSubIdx = 0
		}
	} else if hasAnyHP {
		// Keep old behavior: LP query with HP data defaults to HP(1,1).
		effIsHP = true
		effSubIdx = 0
	}

	buf := fastRichRangeSlicePool.Get().(*[]zmap3base.RichRange)
	spans := (*buf)[:0]

	spans = appendLPSourceRangesFast(g, d, rc.CellIdx, terrain, spans)
	if effIsHP {
		spans = appendHPSourceRangesFast(g, d, rc.CellIdx, effSubIdx, terrain, spans)
	}

	sortSpansByEndBegin(spans)
	result, ok = getIntervalFromTerrainAndSpans(
		terrain,
		spans,
		curY,
		ignoreTexture,
		forbiddenTexture,
		height,
		upLimit,
		downLimit,
	)

	recycleFastSpanBuf(buf, spans)
	return
}

func recycleFastSpanBuf(buf *[]zmap3base.RichRange, spans []zmap3base.RichRange) {
	if cap(spans) > 2048 {
		*buf = make([]zmap3base.RichRange, 0, 96)
	} else {
		*buf = spans[:0]
	}
	fastRichRangeSlicePool.Put(buf)
}

func resolveTerrainFast(g *zmap3base.GridRBData, d *zmap3base.RichRangeSetData, cellIdx int) (zmap3base.RichRange, bool) {
	r := d.RootNode
	if r == 0 {
		return zmap3base.RichRange{}, false
	}
	if zmap3base.IsDirtyEncodedRoot(r) {
		return terrainFromDirtyRootFast(g, r)
	}
	return terrainFromBaseSlice(g.BaseLPOf(cellIdx))
}

func terrainFromDirtyRootFast(g *zmap3base.GridRBData, encodedRoot int32) (zmap3base.RichRange, bool) {
	var (
		bestBase    zmap3base.RichRange
		bestBaseEnd uint16
		foundBase   bool

		bestAny    zmap3base.RichRange
		bestAnyEnd uint16
		foundAny   bool
	)

	for cur := g.DirtyFirstInOrderNode(encodedRoot); cur != zmap3base.NilIdx(); cur = g.DirtyNextInOrderNode(cur) {
		rr, ok := g.DirtyNodeRichRange(cur)
		if !ok {
			break
		}
		if rr.Begin > 0 {
			break
		}
		if rr.End == 0 || rr.End <= rr.Begin {
			continue
		}

		if !foundAny || rr.End > bestAnyEnd {
			bestAny = rr
			bestAnyEnd = rr.End
			foundAny = true
		}
		if (rr.Accessory.Texture&zmap3base.TextureMaterBase) != 0 &&
			(!foundBase || rr.End > bestBaseEnd) {
			bestBase = rr
			bestBaseEnd = rr.End
			foundBase = true
		}
	}

	if foundBase {
		return bestBase, true
	}
	return bestAny, foundAny
}

func appendLPSourceRangesFast(
	g *zmap3base.GridRBData,
	d *zmap3base.RichRangeSetData,
	cellIdx int,
	terrain zmap3base.RichRange,
	out []zmap3base.RichRange,
) []zmap3base.RichRange {
	r := d.RootNode
	switch {
	case r == 0:
		return out // override-empty
	case zmap3base.IsDirtyEncodedRoot(r):
		return appendDirtyRootRangesFast(g, r, terrain, out)
	default:
		return appendBasePayloadFast(g.BaseLPOf(cellIdx), terrain, out)
	}
}

func appendHPSourceRangesFast(
	g *zmap3base.GridRBData,
	d *zmap3base.RichRangeSetData,
	cellIdx int,
	subIdx int,
	terrain zmap3base.RichRange,
	out []zmap3base.RichRange,
) []zmap3base.RichRange {
	baseSeg := g.BaseHPOf(cellIdx, subIdx)
	if d.HighPrecision == nil || !d.HighPrecision.HasSpan(subIdx) {
		return appendBasePayloadFast(baseSeg, terrain, out)
	}

	rootPtr := d.HighPrecision.RootPtrOf(subIdx)
	if rootPtr == nil {
		return appendBasePayloadFast(baseSeg, terrain, out)
	}

	r := *rootPtr
	switch {
	case r == 0:
		return out // override-empty
	case r == zmap3base.NilIdx():
		return appendBasePayloadFast(baseSeg, terrain, out)
	case zmap3base.IsDirtyEncodedRoot(r):
		return appendDirtyRootRangesFast(g, r, terrain, out)
	default:
		return appendBasePayloadFast(baseSeg, terrain, out)
	}
}

func appendBasePayloadFast(seg []zmap3base.RichRange, terrain zmap3base.RichRange, out []zmap3base.RichRange) []zmap3base.RichRange {
	if len(seg) == 0 {
		return out
	}
	start := 0
	if int(seg[0].Begin) == len(seg) {
		start = 1
	}
	for i := start; i < len(seg); i++ {
		rr := seg[i]
		if rr.End == 0 || rr.End <= rr.Begin {
			continue
		}
		if rr.Range == terrain.Range && rr.Accessory == terrain.Accessory {
			continue
		}
		out = append(out, rr)
	}
	return out
}

func appendDirtyRootRangesFast(g *zmap3base.GridRBData, encodedRoot int32, terrain zmap3base.RichRange, out []zmap3base.RichRange) []zmap3base.RichRange {
	for cur := g.DirtyFirstInOrderNode(encodedRoot); cur != zmap3base.NilIdx(); cur = g.DirtyNextInOrderNode(cur) {
		rr, ok := g.DirtyNodeRichRange(cur)
		if !ok {
			break
		}
		if rr.End == 0 || rr.End <= rr.Begin {
			continue
		}
		if rr.Range == terrain.Range && rr.Accessory == terrain.Accessory {
			continue
		}
		out = append(out, rr)
	}
	return out
}
