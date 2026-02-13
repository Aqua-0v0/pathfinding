package navgation

import (
	"math"

	zmap3base "pathfinding/new_map"
)

type Filter struct {
	ignoreTexture,
	forbiddenTexture uint32
	height, upLimit, downLimit int32
}

// GetInterval returns the first traversable gap relative to curY.
// The behavior is aligned with map_data.GetInterval.
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

	terrain, spans, ok := env.GetTerrainAndSpansByPoint(p2d)
	if !ok {
		return
	}

	return getIntervalFromTerrainAndSpans(
		terrain,
		spans,
		curY,
		ignoreTexture,
		forbiddenTexture,
		height,
		upLimit,
		downLimit,
	)
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

	cY := curY
	minAllowed := cY - downLimit
	maxAllowed := cY + upLimit

	gapMinY := terrain.End
	gapTexture := terrain.Accessory.Texture

	for i := 0; i < spansLen; i++ {
		v := spans[i]
		vTexture := v.Accessory.Texture

		if ignore != 0 && (vTexture&ignore) != 0 {
			continue
		}

		if v.End <= gapMinY {
			continue
		}

		if v.Begin < gapMinY {
			gapMinY = v.End
			gapTexture = vTexture
			continue
		}

		gapMaxY := v.Begin

		if gapMaxY-gapMinY >= uint16(height) {
			if int32(gapMaxY) >= needTopY {
				if forbidden == 0 || (gapTexture&forbidden) == 0 {
					gMinI := int32(gapMinY)

					if gMinI > maxAllowed {
						break
					}
					if gMinI >= minAllowed {
						result.Range.Begin = gapMinY
						result.Range.End = gapMaxY
						result.Texture = gapTexture
						return result, true
					}
				}
			}
		}

		gapMinY = v.End
		gapTexture = vTexture

		if int32(gapMinY) > maxAllowed {
			break
		}
	}

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
	if gMinI > maxAllowed {
		return
	}
	if gMinI < minAllowed {
		return
	}

	result.Range.Begin = gapMinY
	result.Range.End = uint16(finalMax)
	result.Texture = gapTexture
	return result, true
}

func (f Filter) GetInterval(
	env *zmap3base.Env,
	p2d zmap3base.Point2d,
	curY int32,
) (result zmap3base.SnapRichRange, ok bool) {
	return GetInterval(
		env,
		p2d,
		curY,
		f.ignoreTexture,
		f.forbiddenTexture,
		f.height,
		f.upLimit,
		f.downLimit,
	)
}
