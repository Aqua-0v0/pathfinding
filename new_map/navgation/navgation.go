package navgation

import (
	"math"
	"sort"
	"sync"

	zmap3base "pathfinding/new_map"
)

type Filter struct {
	ignoreTexture,
	forbiddenTexture uint32
	height, upLimit, downLimit int32
}

var richRangeSlicePool = sync.Pool{
	New: func() any {
		buf := make([]zmap3base.RichRange, 0, 32)
		return &buf
	},
}

// GetInterval returns the first traversable gap relative to curY.
// It is performance-sensitive and intentionally minimizes heap allocation.
func GetInterval(
	env *zmap3base.Env,
	p2d zmap3base.Point2d,
	curY int32,
	ignoreTexture, forbiddenTexture uint32,
	height, upLimit, downLimit int32,
) (result zmap3base.SnapRichRange, ok bool) {
	if env == nil {
		return
	}

	buf := richRangeSlicePool.Get().(*[]zmap3base.RichRange)
	spans := (*buf)[:0]

	terrain, spans, ok := collectTerrainAndSpans(env, p2d, spans)
	if !ok {
		recycleSpanBuf(buf, spans)
		return
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

	recycleSpanBuf(buf, spans)
	return
}

func recycleSpanBuf(buf *[]zmap3base.RichRange, spans []zmap3base.RichRange) {
	if cap(spans) > 1024 {
		*buf = make([]zmap3base.RichRange, 0, 64)
	} else {
		*buf = spans[:0]
	}
	richRangeSlicePool.Put(buf)
}

func collectTerrainAndSpans(
	env *zmap3base.Env,
	p2d zmap3base.Point2d,
	spans []zmap3base.RichRange,
) (terrain zmap3base.RichRange, out []zmap3base.RichRange, ok bool) {
	rc, ok := env.Route(p2d)
	if !ok {
		return zmap3base.RichRange{}, nil, false
	}

	g := rc.G
	d := g.CellByIdx(rc.CellIdx)
	if d == nil {
		return zmap3base.RichRange{}, nil, false
	}

	terrain, ok = resolveTerrain(g, d, rc.CellIdx)
	if !ok {
		return zmap3base.RichRange{}, nil, false
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
		// Align behavior with existing Env logic: LP query with HP present defaults to HP(1,1).
		effIsHP = true
		effSubIdx = 0
	}

	spans = appendLPSourceRanges(g, d, rc.CellIdx, terrain, spans)
	if effIsHP {
		spans = appendHPSourceRanges(g, d, rc.CellIdx, effSubIdx, terrain, spans)
	}

	return terrain, spans, true
}

func resolveTerrain(g *zmap3base.GridRBData, d *zmap3base.RichRangeSetData, cellIdx int) (zmap3base.RichRange, bool) {
	r := d.RootNode
	if r == 0 {
		return zmap3base.RichRange{}, false
	}

	if zmap3base.IsDirtyEncodedRoot(r) {
		return terrainFromDirtyRoot(g, r)
	}

	// For base root and nil root, both fallback to base LP slice.
	return terrainFromBaseSlice(g.BaseLPOf(cellIdx))
}

func terrainFromBaseSlice(seg []zmap3base.RichRange) (zmap3base.RichRange, bool) {
	if len(seg) == 0 {
		return zmap3base.RichRange{}, false
	}

	if int(seg[0].Begin) == len(seg) {
		hdr := seg[0]
		if hdr.End > 0 {
			rr := hdr
			rr.Range = zmap3base.Range{Begin: 0, End: hdr.End}
			return rr, true
		}
	}

	var (
		best    zmap3base.RichRange
		bestEnd uint16
		found   bool
	)

	for i := 0; i < len(seg); i++ {
		rr := seg[i]
		if rr.Begin != 0 || rr.End == 0 || rr.End <= rr.Begin {
			continue
		}
		if (rr.Accessory.Texture & zmap3base.TextureMaterBase) == 0 {
			continue
		}
		if !found || rr.End > bestEnd {
			best = rr
			bestEnd = rr.End
			found = true
		}
	}
	if found {
		return best, true
	}

	for i := 0; i < len(seg); i++ {
		rr := seg[i]
		if rr.Begin != 0 || rr.End == 0 || rr.End <= rr.Begin {
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

func terrainFromDirtyRoot(g *zmap3base.GridRBData, encodedRoot int32) (zmap3base.RichRange, bool) {
	t := g.Ops().TreeFromEncodedRoot(encodedRoot)

	q := zmap3base.Range{Begin: 0, End: 1}
	var (
		best    zmap3base.RichRange
		bestEnd uint16
		found   bool
	)

	t.RangeQueryInOrder(q, func(rr zmap3base.RichRange) bool {
		if rr.Begin != 0 || rr.End == 0 || rr.End <= rr.Begin {
			return true
		}
		if (rr.Accessory.Texture & zmap3base.TextureMaterBase) == 0 {
			return true
		}
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

	t.RangeQueryInOrder(q, func(rr zmap3base.RichRange) bool {
		if rr.Begin != 0 || rr.End == 0 || rr.End <= rr.Begin {
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

func appendLPSourceRanges(
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
		return appendDirtyRootRanges(g, r, terrain, out)
	default:
		// base/nil/unknown => fallback base slice
		return appendBasePayload(g.BaseLPOf(cellIdx), terrain, out)
	}
}

func appendHPSourceRanges(
	g *zmap3base.GridRBData,
	d *zmap3base.RichRangeSetData,
	cellIdx int,
	subIdx int,
	terrain zmap3base.RichRange,
	out []zmap3base.RichRange,
) []zmap3base.RichRange {
	baseSeg := g.BaseHPOf(cellIdx, subIdx)

	if d.HighPrecision == nil || !d.HighPrecision.HasSpan(subIdx) {
		return appendBasePayload(baseSeg, terrain, out)
	}

	rootPtr := d.HighPrecision.RootPtrOf(subIdx)
	if rootPtr == nil {
		return appendBasePayload(baseSeg, terrain, out)
	}

	r := *rootPtr
	switch {
	case r == 0:
		return out // override-empty
	case r == zmap3base.NilIdx():
		return appendBasePayload(baseSeg, terrain, out)
	case zmap3base.IsDirtyEncodedRoot(r):
		return appendDirtyRootRanges(g, r, terrain, out)
	default:
		// base/unknown => fallback base slice
		return appendBasePayload(baseSeg, terrain, out)
	}
}

func appendBasePayload(seg []zmap3base.RichRange, terrain zmap3base.RichRange, out []zmap3base.RichRange) []zmap3base.RichRange {
	if len(seg) == 0 {
		return out
	}

	start := 0
	if int(seg[0].Begin) == len(seg) {
		start = 1 // header
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

func appendDirtyRootRanges(g *zmap3base.GridRBData, encodedRoot int32, terrain zmap3base.RichRange, out []zmap3base.RichRange) []zmap3base.RichRange {
	t := g.Ops().TreeFromEncodedRoot(encodedRoot)
	t.ForeachAll(func(rr zmap3base.RichRange) bool {
		if rr.End == 0 || rr.End <= rr.Begin {
			return true
		}
		if rr.Range == terrain.Range && rr.Accessory == terrain.Accessory {
			return true
		}
		out = append(out, rr)
		return true
	})
	return out
}

func sortSpansByEndBegin(spans []zmap3base.RichRange) {
	n := len(spans)
	if n < 2 {
		return
	}

	// Small n dominates hot path; insertion sort is faster and allocation-free.
	if n <= 24 {
		for i := 1; i < n; i++ {
			x := spans[i]
			j := i - 1
			for ; j >= 0; j-- {
				if spans[j].End < x.End || (spans[j].End == x.End && spans[j].Begin <= x.Begin) {
					break
				}
				spans[j+1] = spans[j]
			}
			spans[j+1] = x
		}
		return
	}

	sort.Slice(spans, func(i, j int) bool {
		if spans[i].End == spans[j].End {
			return spans[i].Begin < spans[j].Begin
		}
		return spans[i].End < spans[j].End
	})
}

func getIntervalFromTerrainAndSpans(
	terrain zmap3base.RichRange,
	spans []zmap3base.RichRange,
	curY int32,
	ignoreTexture, forbiddenTexture uint32,
	height, upLimit, downLimit int32,
) (result zmap3base.SnapRichRange, ok bool) {
	spansLen := len(spans)

	ignore := zmap3base.Texture(ignoreTexture)
	forbidden := zmap3base.Texture(forbiddenTexture)

	if spansLen == 0 {
		gapTexture := terrain.Accessory.Texture
		if forbidden != 0 && (gapTexture&forbidden) != 0 {
			return
		}

		tMax := int32(terrain.End)
		cY := curY
		if tMax > cY+upLimit {
			return
		}
		if tMax < cY-downLimit {
			return
		}

		result.Range.Begin = terrain.End
		result.Range.End = uint16(math.MaxUint16)
		result.Texture = gapTexture
		return result, true
	}

	needTopY := curY + height
	minAllowed := curY - downLimit
	maxAllowed := curY + upLimit
	h16 := uint16(height)

	gapMinY := terrain.End
	gapTexture := terrain.Accessory.Texture

	if ignore == 0 {
		if forbidden == 0 {
			return intervalI0F0(spans, gapMinY, gapTexture, needTopY, minAllowed, maxAllowed, h16, height)
		}
		return intervalI0F1(spans, gapMinY, gapTexture, forbidden, needTopY, minAllowed, maxAllowed, h16, height)
	}
	return intervalGeneral(spans, gapMinY, gapTexture, ignore, forbidden, needTopY, minAllowed, maxAllowed, h16, height)
}

func intervalI0F0(
	spans []zmap3base.RichRange,
	gapMinY uint16,
	gapTexture zmap3base.Texture,
	needTopY, minAllowed, maxAllowed int32,
	h16 uint16,
	height int32,
) (result zmap3base.SnapRichRange, ok bool) {
	for i := 0; i < len(spans); i++ {
		v := spans[i]
		if v.End <= gapMinY {
			continue
		}

		if v.Begin >= gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				break
			}
			if gMinI >= minAllowed {
				gapMaxY := v.Begin
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.Range.Begin = gapMinY
					result.Range.End = gapMaxY
					result.Texture = gapTexture
					return result, true
				}
			}
		}

		gapMinY = v.End
		gapTexture = v.Accessory.Texture
		if int32(gapMinY) > maxAllowed {
			break
		}
	}

	return finalGap(gapMinY, gapTexture, needTopY, minAllowed, maxAllowed, height, 0)
}

func intervalI0F1(
	spans []zmap3base.RichRange,
	gapMinY uint16,
	gapTexture zmap3base.Texture,
	forbidden zmap3base.Texture,
	needTopY, minAllowed, maxAllowed int32,
	h16 uint16,
	height int32,
) (result zmap3base.SnapRichRange, ok bool) {
	for i := 0; i < len(spans); i++ {
		v := spans[i]
		if v.End <= gapMinY {
			continue
		}

		if v.Begin >= gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				break
			}
			if gMinI >= minAllowed {
				gapMaxY := v.Begin
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTexture&forbidden) == 0 {
					result.Range.Begin = gapMinY
					result.Range.End = gapMaxY
					result.Texture = gapTexture
					return result, true
				}
			}
		}

		gapMinY = v.End
		gapTexture = v.Accessory.Texture
		if int32(gapMinY) > maxAllowed {
			break
		}
	}

	return finalGap(gapMinY, gapTexture, needTopY, minAllowed, maxAllowed, height, forbidden)
}

func intervalGeneral(
	spans []zmap3base.RichRange,
	gapMinY uint16,
	gapTexture zmap3base.Texture,
	ignore, forbidden zmap3base.Texture,
	needTopY, minAllowed, maxAllowed int32,
	h16 uint16,
	height int32,
) (result zmap3base.SnapRichRange, ok bool) {
	for i := 0; i < len(spans); i++ {
		v := spans[i]
		vTex := v.Accessory.Texture
		if (vTex & ignore) != 0 {
			continue
		}
		if v.End <= gapMinY {
			continue
		}

		if v.Begin >= gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				break
			}
			if gMinI >= minAllowed {
				gapMaxY := v.Begin
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbidden == 0 || (gapTexture&forbidden) == 0) {
					result.Range.Begin = gapMinY
					result.Range.End = gapMaxY
					result.Texture = gapTexture
					return result, true
				}
			}
		}

		gapMinY = v.End
		gapTexture = vTex
		if int32(gapMinY) > maxAllowed {
			break
		}
	}

	return finalGap(gapMinY, gapTexture, needTopY, minAllowed, maxAllowed, height, forbidden)
}

func finalGap(
	gapMinY uint16,
	gapTexture zmap3base.Texture,
	needTopY, minAllowed, maxAllowed int32,
	height int32,
	forbidden zmap3base.Texture,
) (result zmap3base.SnapRichRange, ok bool) {
	finalMax := int32(math.MaxUint16)
	if finalMax-int32(gapMinY) < height {
		return
	}
	if finalMax < needTopY {
		return
	}
	if forbidden != 0 && (gapTexture&forbidden) != 0 {
		return
	}

	gMinI := int32(gapMinY)
	if gMinI > maxAllowed || gMinI < minAllowed {
		return
	}

	result.Range.Begin = gapMinY
	result.Range.End = uint16(finalMax)
	result.Texture = gapTexture
	return result, true
}
