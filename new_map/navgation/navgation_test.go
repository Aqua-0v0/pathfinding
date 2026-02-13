package navgation

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
	"unsafe"

	"pathfinding/map_data"
	zmap3base "pathfinding/new_map"
)

const (
	testTexBase  = zmap3base.TextureMaterBase
	testTexObs   = zmap3base.TextureMaterObstacle
	testTexCol   = zmap3base.TextureMaterCollider
	testTexWater = zmap3base.TexturePropWater
)

var (
	benchResult zmap3base.SnapRichRange
	benchOK     bool
)

type cellFixture struct {
	terrain   zmap3base.RichRange
	lpPayload []zmap3base.RichRange
	hpPayload map[int][]zmap3base.RichRange
}

func rr(begin, end uint16, tex zmap3base.Texture) zmap3base.RichRange {
	return zmap3base.RichRange{
		Range: zmap3base.Range{
			Begin: begin,
			End:   end,
		},
		Accessory: zmap3base.Accessory{
			Texture: tex,
		},
	}
}

func buildSingleGridEnv(t testing.TB, cells map[int]cellFixture) *zmap3base.Env {
	t.Helper()

	lpPerCell := make([][]zmap3base.RichRange, zmap3base.FastGridCellNum)
	hpPerCell := make([][zmap3base.SecondaryTileNum][]zmap3base.RichRange, zmap3base.FastGridCellNum)

	for idx, c := range cells {
		lp := make([]zmap3base.RichRange, 0, 1+len(c.lpPayload))
		lp = append(lp, c.terrain)
		lp = append(lp, c.lpPayload...)
		lpPerCell[idx] = lp

		for sub, payload := range c.hpPayload {
			if sub < 0 || sub >= zmap3base.SecondaryTileNum {
				t.Fatalf("invalid hp subIdx: %d", sub)
			}
			cp := append([]zmap3base.RichRange(nil), payload...)
			hpPerCell[idx][sub] = cp
		}
	}

	grid, err := zmap3base.BuildGridRBDataFromSlices(0, 0, lpPerCell, hpPerCell)
	if err != nil {
		t.Fatalf("BuildGridRBDataFromSlices failed: %v", err)
	}

	env := zmap3base.NewEnv(zmap3base.Rect{
		Min: zmap3base.Point2d{X: 0, Y: 0},
		Max: zmap3base.Point2d{X: zmap3base.FastGridSetSize, Y: zmap3base.FastGridSetSize},
	})
	injectSingleGridForTest(t, env, grid)
	return env
}

func injectSingleGridForTest(t testing.TB, env *zmap3base.Env, grid *zmap3base.GridRBData) {
	t.Helper()
	if env == nil {
		t.Fatalf("env is nil")
	}
	if grid == nil {
		t.Fatalf("grid is nil")
	}

	v := reflect.ValueOf(env).Elem().FieldByName("grids")
	if !v.IsValid() || v.Len() == 0 {
		t.Fatalf("env.grids not found or empty")
	}

	grids := make([]*zmap3base.GridRBData, v.Len())
	grids[0] = grid

	ptr := reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	ptr.Set(reflect.ValueOf(grids))
}

func TestGetIntervalCore_EmptySpans(t *testing.T) {
	terrain := rr(0, 120, testTexBase)

	got, ok := getIntervalFromTerrainAndSpans(
		terrain,
		nil,
		120,
		0, 0,
		10,
		200,
		200,
	)
	if !ok {
		t.Fatalf("expected interval, got none")
	}
	if got.Begin != 120 || got.End != uint16(math.MaxUint16) || got.Texture != testTexBase {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestGetIntervalCore_EmptySpansForbiddenTexture(t *testing.T) {
	terrain := rr(0, 50, testTexBase|testTexWater)

	_, ok := getIntervalFromTerrainAndSpans(
		terrain,
		nil,
		50,
		0,
		uint32(testTexWater),
		10,
		200,
		200,
	)
	if ok {
		t.Fatalf("expected no interval due to forbidden terrain texture")
	}
}

func TestGetIntervalCore_IgnoreAndForbidden(t *testing.T) {
	terrain := rr(0, 0, testTexBase)
	spans := []zmap3base.RichRange{
		rr(20, 25, testTexBase),
		rr(0, 5, testTexObs),
	}
	sortSpansByEndBegin(spans)

	t.Run("forbidden-skip-middle-gap", func(t *testing.T) {
		got, ok := getIntervalFromTerrainAndSpans(
			terrain,
			spans,
			5,
			0,
			uint32(testTexObs),
			2,
			200,
			200,
		)
		if !ok {
			t.Fatalf("expected interval, got none")
		}
		if got.Begin != 25 || got.End != uint16(math.MaxUint16) || got.Texture != testTexBase {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("ignore-first-span", func(t *testing.T) {
		got, ok := getIntervalFromTerrainAndSpans(
			terrain,
			spans,
			1,
			uint32(testTexObs),
			0,
			2,
			200,
			200,
		)
		if !ok {
			t.Fatalf("expected interval, got none")
		}
		if got.Begin != 0 || got.End != 20 || got.Texture != testTexBase {
			t.Fatalf("unexpected result: %+v", got)
		}
	})
}

func TestGetIntervalCore_UpDownLimits(t *testing.T) {
	terrain := rr(0, 10, testTexBase)

	_, ok := getIntervalFromTerrainAndSpans(
		terrain,
		nil,
		30,
		0,
		0,
		5,
		100,
		5,
	)
	if ok {
		t.Fatalf("expected no interval due to downLimit")
	}

	_, ok = getIntervalFromTerrainAndSpans(
		terrain,
		nil,
		0,
		0,
		0,
		5,
		3,
		100,
	)
	if ok {
		t.Fatalf("expected no interval due to upLimit")
	}
}

func TestGetInterval_PublicLPQueryUsesDefaultHP11(t *testing.T) {
	cellIdx := 1 + 1*zmap3base.FastGridSetSize
	env := buildSingleGridEnv(t, map[int]cellFixture{
		cellIdx: {
			terrain: rr(0, 10, testTexBase),
			hpPayload: map[int][]zmap3base.RichRange{
				0: {
					rr(10, 40, testTexCol),
				},
			},
		},
	})

	got, ok := GetInterval(
		env,
		zmap3base.Point2d{X: 1, Y: 1},
		10,
		0,
		0,
		2,
		200,
		200,
	)
	if !ok {
		t.Fatalf("expected interval, got none")
	}
	if got.Begin != 40 || got.End != uint16(math.MaxUint16) || got.Texture != testTexCol {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestGetInterval_PublicHPQueryFallbackToLPWhenNoHP(t *testing.T) {
	cellIdx := 2 + 2*zmap3base.FastGridSetSize
	env := buildSingleGridEnv(t, map[int]cellFixture{
		cellIdx: {
			terrain: rr(0, 10, testTexBase),
			lpPayload: []zmap3base.RichRange{
				rr(10, 22, testTexObs),
			},
		},
	})

	got, ok := GetInterval(
		env,
		zmap3base.Point2d{X: 2, Y: 2, XOffset: 1, YOffset: 1},
		10,
		0,
		0,
		2,
		200,
		200,
	)
	if !ok {
		t.Fatalf("expected interval, got none")
	}
	if got.Begin != 22 || got.End != uint16(math.MaxUint16) || got.Texture != testTexObs {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestGetInterval_PublicInvalidPoint(t *testing.T) {
	env := buildSingleGridEnv(t, map[int]cellFixture{
		0: {
			terrain: rr(0, 8, testTexBase),
		},
	})

	_, ok := GetInterval(
		env,
		zmap3base.Point2d{X: 40, Y: 40},
		8,
		0,
		0,
		2,
		200,
		200,
	)
	if ok {
		t.Fatalf("expected no interval for out-of-rect point")
	}
}

func TestGetIntervalCore_RandomizedAgainstMapDataNonEmptySpans(t *testing.T) {
	rng := rand.New(rand.NewSource(20260213))
	textures := []zmap3base.Texture{
		testTexBase,
		testTexObs,
		testTexCol,
		testTexBase | testTexWater,
	}

	for i := 0; i < 2000; i++ {
		terrainEnd := uint16(1 + rng.Intn(300))
		terrainTex := textures[rng.Intn(len(textures))]
		terrain := rr(0, terrainEnd, terrainTex)

		n := 1 + rng.Intn(8) // non-empty by design
		spans := make([]zmap3base.RichRange, 0, n)
		for j := 0; j < n; j++ {
			begin := uint16(rng.Intn(420))
			end := begin + uint16(1+rng.Intn(50))
			spans = append(spans, rr(begin, end, textures[rng.Intn(len(textures))]))
		}
		sortSpansByEndBegin(spans)

		curY := int32(rng.Intn(420))
		height := int32(1 + rng.Intn(32))
		upLimit := int32(20 + rng.Intn(240))
		downLimit := int32(20 + rng.Intn(240))

		var ignore uint32
		if rng.Intn(3) == 0 {
			ignore = uint32(textures[rng.Intn(len(textures))])
		}
		var forbidden uint32
		if rng.Intn(3) == 0 {
			forbidden = uint32(textures[rng.Intn(len(textures))])
		}

		got, gotOK := getIntervalFromTerrainAndSpans(
			terrain,
			spans,
			curY,
			ignore,
			forbidden,
			height,
			upLimit,
			downLimit,
		)

		mTerrain := map_data.Span{
			MinY:    0,
			MaxY:    terrain.End,
			Texture: uint32(terrain.Accessory.Texture),
		}
		mSpans := make([]map_data.Span, len(spans))
		for k := 0; k < len(spans); k++ {
			mSpans[k] = map_data.Span{
				MinY:    spans[k].Begin,
				MaxY:    spans[k].End,
				Texture: uint32(spans[k].Accessory.Texture),
			}
		}
		want, wantOK := map_data.GetInterval(
			mTerrain,
			mSpans,
			curY,
			ignore,
			forbidden,
			height,
			upLimit,
			downLimit,
		)

		if gotOK != wantOK {
			t.Fatalf("ok mismatch case=%d got=%v want=%v terrain=%+v curY=%d h=%d up=%d down=%d ignore=%d forbidden=%d spans=%+v",
				i, gotOK, wantOK, terrain, curY, height, upLimit, downLimit, ignore, forbidden, spans)
		}
		if gotOK {
			if got.Begin != want.MinY || got.End != want.MaxY || uint32(got.Texture) != want.Texture {
				t.Fatalf("result mismatch case=%d got=%+v want={MinY:%d MaxY:%d Texture:%d} terrain=%+v curY=%d h=%d up=%d down=%d ignore=%d forbidden=%d spans=%+v",
					i, got, want.MinY, want.MaxY, want.Texture, terrain, curY, height, upLimit, downLimit, ignore, forbidden, spans)
			}
		}
	}
}

func BenchmarkGetIntervalCore(b *testing.B) {
	terrain := rr(0, 320, testTexBase)
	spans := []zmap3base.RichRange{
		rr(0, 40, testTexObs),
		rr(46, 68, testTexCol),
		rr(80, 95, testTexObs),
		rr(101, 125, testTexBase|testTexWater),
		rr(140, 153, testTexCol),
		rr(171, 196, testTexObs),
		rr(210, 230, testTexBase),
		rr(252, 276, testTexCol),
	}
	sortSpansByEndBegin(spans)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		curY := int32(60 + (i & 15))
		benchResult, benchOK = getIntervalFromTerrainAndSpans(
			terrain,
			spans,
			curY,
			0,
			0,
			12,
			160,
			120,
		)
	}
}

func BenchmarkGetIntervalPublic(b *testing.B) {
	cellIdx := 4 + 4*zmap3base.FastGridSetSize
	env := buildSingleGridEnv(b, map[int]cellFixture{
		cellIdx: {
			terrain: rr(0, 40, testTexBase),
			lpPayload: []zmap3base.RichRange{
				rr(40, 56, testTexObs),
				rr(62, 83, testTexCol),
			},
			hpPayload: map[int][]zmap3base.RichRange{
				0: {
					rr(86, 106, testTexObs),
					rr(119, 142, testTexCol),
				},
			},
		},
	})

	p := zmap3base.Point2d{X: 4, Y: 4} // LP query with HP present => default HP(1,1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		curY := int32(40 + (i & 7))
		benchResult, benchOK = GetInterval(
			env,
			p,
			curY,
			0,
			0,
			10,
			200,
			200,
		)
	}
}
