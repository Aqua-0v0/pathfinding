package map_data

import "math"

// 1B == 24.4M

type Vector3Float struct {
	X float32 `protobuf:"fixed32,1,opt,name=x,proto3" json:"x,omitempty"`
	Y float32 `protobuf:"fixed32,2,opt,name=y,proto3" json:"y,omitempty"`
	Z float32 `protobuf:"fixed32,3,opt,name=z,proto3" json:"z,omitempty"`
}

const FastGridSetSize = 32 // 必须为2的n次幂

type Chunk [FastGridSetSize * FastGridSetSize]Column

type Space struct {
	Map    []*Chunk
	minX   uint16
	minZ   uint16
	width  uint16
	height uint16
}

func (s *Space) InitWidthAndHeight(spaceWidth, spaceHeight uint16) {
	s.width = (spaceWidth + FastGridSetSize - 1) / FastGridSetSize
	s.height = (spaceHeight + FastGridSetSize - 1) / FastGridSetSize
}

func (s *Space) GetColum(p Vector3Float) (c Column, ok bool) {
	var x, z = uint16(p.X) - s.minX, uint16(p.Y) - s.minZ
	level1 := x/FastGridSetSize + (z/FastGridSetSize)*s.width
	if int(level1) >= len(s.Map) {
		return c, false
	}
	fstGridSet := s.Map[level1]
	if fstGridSet == nil {
		return c, false
	} else {
		//fgd = fstGridSet[ox%FastGridSetSize+(oy%FastGridSetSize)*FastGridSetSize]
		c = fstGridSet[x&(FastGridSetSize-1)+(z&(FastGridSetSize-1))*FastGridSetSize]
		return
	}
}

type Mask uint32

// ColumnIsTerrain 这一处1x1的柱列紧紧只有一块地形数据
func (s Mask) ColumnIsTerrain() bool {
	return s&1 > 0
}

// ColumnIsLowPrecision 这一处1x1的柱列是低精度的，代表这1x1的柱列仅有地形数据以及Other数据
func (s Mask) ColumnIsLowPrecision() bool {
	return s&2 > 0
}

// Column 这一处1x1的柱列信息
type Column struct {
	Mask          uint32               // isTerrain  isLowPrecision
	Terrain       Span                 // 地形数据, 一定存在地形，地形数据仅为低精度1x1信息
	Other         *[]Span              // 如果是低精度的柱列，除了地形的柱列信息
	HighPrecision *HighPrecisionColumn // 如果是高精度的柱列，除了地形的柱列信息
}

type Same [5]uint16

type HighPrecisionColumn struct {
	Same  Same
	Spans [][]Span
}

// GetSpans 获取这个坐标对应的地形及阻挡数据。
// 性能敏感函数，不要动
// 速度1ns-6.2ns, 取决于命中位置
func (s *Column) GetSpans(p PointForSearchPath) (Span, []Span) {
	// --------------------------------- lowPrecision ----------------------------

	// inline: ColumnIsTerrain()
	if s.Mask&1 > 0 {
		return s.Terrain, nil
	}
	// inline: ColumnIsLowPrecision()
	if s.Mask&2 > 0 {
		return s.Terrain, *s.Other
	}

	// ---------------------------------  highPrecision ----------------------------

	spans := s.HighPrecision.Spans

	g := p.GridIndex
	same := s.HighPrecision.Same

	lo := uint64(same[0]) | (uint64(same[1]) << 16) | (uint64(same[2]) << 32) | (uint64(same[3]) << 48)
	hi := uint64(same[4])

	var idx uint8 = 0

	// group0 start=0: flag=bit0, grid=bit1..4
	// v0 := lo >> 0
	// grid0 = (v0>>1)&0xF
	if uint16((lo>>1)&0x0F) == g {
		return s.Terrain, spans[0]
	}

	// group1 start=5
	v1 := lo >> 5
	idx += uint8(v1 & 1)
	if uint16((v1>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group2 start=10
	v2 := lo >> 10
	idx += uint8(v2 & 1)
	if uint16((v2>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group3 start=15
	v3 := lo >> 15
	idx += uint8(v3 & 1)
	if uint16((v3>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group4 start=20
	v4 := lo >> 20
	idx += uint8(v4 & 1)
	if uint16((v4>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group5 start=25
	v5 := lo >> 25
	idx += uint8(v5 & 1)
	if uint16((v5>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group6 start=30
	v6 := lo >> 30
	idx += uint8(v6 & 1)
	if uint16((v6>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group7 start=35
	v7 := lo >> 35
	idx += uint8(v7 & 1)
	if uint16((v7>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group8 start=40
	v8 := lo >> 40
	idx += uint8(v8 & 1)
	if uint16((v8>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group9 start=45
	v9 := lo >> 45
	idx += uint8(v9 & 1)
	if uint16((v9>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group10 start=50
	v10 := lo >> 50
	idx += uint8(v10 & 1)
	if uint16((v10>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group11 start=55
	v11 := lo >> 55
	idx += uint8(v11 & 1)
	if uint16((v11>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group12 start=60: nibble跨界
	// flag = lo>>60 &1
	// grid = bits61..64 => (lo>>61)&0x7 | (hi&1)<<3
	v12 := lo >> 60
	idx += uint8(v12 & 1)
	if uint16(((v12>>1)&0x07)|((hi&1)<<3)) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group13 start=65: 全在 hi
	v13 := hi >> 1
	idx += uint8(v13 & 1)
	if uint16((v13>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group14 start=70
	v14 := hi >> 6
	idx += uint8(v14 & 1)
	if uint16((v14>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	// group15 start=75
	v15 := hi >> 11
	idx += uint8(v15 & 1)
	if uint16((v15>>1)&0x0F) == g {
		return s.Terrain, spans[int(idx)]
	}

	return s.Terrain, spans[0] // 兜底
}

// GetInterval 返回相对于当前位置，这个柱列阻挡的可通行的空隙
func GetInterval(
	terrain Span,
	spans []Span,
	curY int32,
	ignoreTexture, forbiddenTexture uint32,
	height, upLimit, downLimit int32,
) (result Span, ok bool) {
	spansLen := len(spans)
	if spansLen == 0 {
		// terrain.MinY 一定为0
		if forbiddenTexture != 0 && (terrain.Texture&forbiddenTexture) != 0 {
			return
		}

		// up/down limit check
		// 统一用 int32，避免溢出
		tMax := int32(terrain.MaxY)
		cY := curY
		if tMax > cY+upLimit {
			return
		}
		if tMax < cY-downLimit {
			return
		}
		return terrain, true
	}

	// 头顶高度要求：gapMaxY >= curY + height
	// 用 uint32 避免溢出
	needTopY := curY + height

	// gapMinY 限制范围
	// 用 int32 避免 (curY - downLimit) 下溢
	cY := curY
	minAllowed := cY - downLimit
	maxAllowed := cY + upLimit

	// --------------------------- init first gap -------------------------------
	gapMinY := terrain.MaxY
	gapTexture := terrain.Texture

	// --------------------------- scan blocking spans --------------------------
	for i := 0; i < spansLen; i++ {
		v := spans[i]

		// ignoreTexture：跳过这段阻挡
		if ignoreTexture != 0 && (v.Texture&ignoreTexture) != 0 {
			continue
		}

		// v 在 gapMinY 下方：无影响
		if v.MaxY <= gapMinY {
			continue
		}

		// v 与 gapMinY 重叠：gap 被吞掉，提升 gapMinY
		if v.MinY < gapMinY {
			gapMinY = v.MaxY
			gapTexture = v.Texture
			continue
		}

		// 形成 gap: [gapMinY, v.MinY)
		gapMaxY := v.MinY

		// 1) gap 高度 >= height
		if gapMaxY-gapMinY >= uint16(height) {

			// 2) gap 顶部 >= needTopY
			if int32(gapMaxY) >= needTopY {

				// 3) forbiddenTexture
				if forbiddenTexture == 0 || (gapTexture&forbiddenTexture) == 0 {

					// 4) up/down limit：只检查 gapMinY
					gMinI := int32(gapMinY)

					// gapMinY 超出上限：后面只会更大，直接 break
					if gMinI > maxAllowed {
						break
					}

					// gapMinY 在允许范围内
					if gMinI >= minAllowed {
						result.MinY = gapMinY
						result.MaxY = gapMaxY
						result.Texture = gapTexture
						return result, true
					}
				}
			}
		}

		// 更新 gap 起点为当前阻挡段上边界
		gapMinY = v.MaxY
		gapTexture = v.Texture

		// 提前 break：gapMinY 已经超过上限，后面更高
		if int32(gapMinY) > maxAllowed {
			break
		}
	}

	// 最后 gap: [gapMinY, MaxUint16)
	finalMax := int32(math.MaxUint16)

	// 1) gap 高度 >= height
	if finalMax-int32(gapMinY) < height {
		return
	}
	// 2) finalMax >= needTopY：必然满足，但保留最少判断成本（可删）
	if finalMax < needTopY {
		return
	}
	// 3) forbiddenTexture
	if forbiddenTexture != 0 && (gapTexture&forbiddenTexture) != 0 {
		return
	}
	// 4) up/down limit
	gMinI := int32(gapMinY)
	if gMinI > maxAllowed {
		return
	}
	if gMinI < minAllowed {
		return
	}

	result.MinY = gapMinY
	result.MaxY = uint16(finalMax)
	result.Texture = gapTexture
	return result, true
}

// Span 阻挡阻挡区间数据及阻挡Texture信息
type Span struct {
	// [minY, maxY)范围内都是阻挡，如果是地形的话，minY必为0，单位为0.05米, 也就是是说高度的精度为0.05米，为了避免运行过程中的浮点数计算，这里使用了uint16，在寻路完成之后，统一进行转换为世界坐标(浮点数)，举例来说，[324,546)代表真实世界的[16.2,27.3)这段高度都是阻挡
	MinY    uint16
	MaxY    uint16
	Texture uint32 // 地形、水面、墙体、门等等信息
}

type PointForSearchPath struct {
	X         uint16 // x的世界坐标整数
	Y         uint16 // y与Span里的Y是相同的单位， y*20则为真实世界坐标
	Z         uint16 // z的世界坐标整数
	XOffSet   uint8  // 这1x1坐标内划分了16相同份，可以理解为一个2维数组[4][4]，XOffSet为一维数组索引
	ZOffSet   uint8  // 这1x1坐标内划分了16相同份，可以理解为一个2维数组[4][4]，ZOffSet为二维数组索引
	GridIndex uint16 // 初始化PointForSearchPath之后要调用CalcGridIndex来设置GridIndex的值，这个值在GetSpans时有用
}

func (s *PointForSearchPath) CalcGridIndex() {
	s.GridIndex = uint16(s.XOffSet)*4 + uint16(s.ZOffSet)
}

// Texture的每一个bit表示一种材质，Texture可能是多种材质复合组成。多种材质复合时，寻路过程如果传入的忽略材质 Texture1&Texture2>0 就会忽略这个Span
var example []Span = []Span{
	{MinY: 0, MaxY: 300, Texture: 1},
	{MinY: 300, MaxY: 340, Texture: 2},
	{MinY: 380, MaxY: 460, Texture: 4},
	{MinY: 400, MaxY: 480, Texture: 8},
	{MinY: 485, MaxY: 600, Texture: 8},
	{MinY: 600, MaxY: 620, Texture: 16},
	{MinY: 680, MaxY: 710, Texture: 4},
}

/*

1.[0,340)这段高度是阻挡，上面有380-340=40高度的空隙，落脚点高度为340，落脚点的texture=2。
2.[380,380)这段高度是阻挡，上面有485-480=5高度的空隙，落脚点高度为380，落脚点的texture=8，如果寻路设置可通行高度1.1m，换算高度22，那么这段空隙不满足身高要求。
3.[485,600)这段高度是阻挡，上面有680-600=80高度的空隙，落脚点高度为600，落脚点的texture=16，因为假设了寻路设置的忽略Texture=16，{MinY: 600, MaxY: 620, Texture: 16}这段Span直接配忽略掉。
4.[680,710)这段高度是阻挡，[710,math.MaxUint16]这段高度是空隙，落脚点高度为680，落脚点的Texture=4。
5.结合之前给的map.go里的注释描述，在寻路时，当前位置落脚点到下一个位置的落脚点的高度如果升高了，那么最高只能升高1.1米(这个值可在寻路函数传参)；当前位置落脚点到下一个位置的落脚点的高度如果下降了，那么最高只能下降65535米(这个值可在寻路函数传参), 并且落脚点对应的空隙的最高点-当前位置的落脚点的值要大于身高，放置碰头;当前位置落脚点到下一个位置的落脚点时，下一个落脚点的空隙最高点-当前位置落脚点要大于物体的高度(物体的高度这个值可在寻路函数传参);当前位置落脚点到下一个位置的落脚点时，某些落脚点的Texture禁止落脚(比如水, 哪些落脚点的Texture不能落脚可在寻路函数传参);可以选择忽略指定类型的Texture阻挡(寻路函数传参这个Texture)；如果起点或者终点在阻挡里，则直接寻路失败；寻路物体的体型x,z固定占用2*2高精格子。
*/

// 基础信息讲解：
// 世界里的所有地形、阻挡信息都记录在Space中，你可以通过InitWidthAndHeight，和GetColum函数来理解	minX、minZ、width、height的含义。
// 每个Chunk单位为32mx32m，InitWidthAndHeight(spaceWidth, spaceHeight uint16)函数里在传参时保证spaceWidth和spaceHeight都必须为32的整数倍。
// 柱列：地形Span+[]Span,表示这个格子内的所有阻挡数据，地形也被认为是阻挡。Span的详细解释需要看Span的注释。[]Span是有序的，优先按Span.MaxY由小到大排序，如果Span.MaxY一样，则Span.MinY由小到大
// 每个Column记录这个1mx1m格子内的所有阻挡信息。每个Column里面存储了4x4=16个柱列的所有阻挡、地形数据。为了减少内存开销，约定一定会有地形数据且地形数据为1mx1m，
// 并且引入了一个我自定的定义：高精、低精。
// 低精就是这个1x1内的16个柱列的阻挡信息都一致或者只有地形信息。这时只读Column.Terrain、Column.Other即可，只有地形时Column.Other为nil。
// 高精就是这个1x1内的16个柱列的阻挡信息部分一致或者都不一致，那么阻挡信息就要从Column.HighPrecisionColumn中获取对应格子的柱列信息，如果读取详见GetSpans函数。无论高精还是低精都可以通过GetSpans函数获取对应格子的柱列信息。
// 通过以上信息，我们也能知道了地图数据的x、z的精度为0.25m,y的精度为0.05m.
// PointForSearchPath为寻路过程中一直使用的点的表示(也可以理解为格子)，在寻路完成之后，统一进行转换为世界坐标(浮点数)。
// 寻路物体的体型x,z固定占用2*2格子，物体的高度可以通过寻路函数来指定，默认1.1m高，3d寻路其实也是8方向连通，判断当前为止与下个位置是否可以通过的逻辑需要满足下面几点要求：
// 当前位置落脚点到下一个位置的落脚点的高度如果升高了，那么最高只能升高1.1米(这个值可在寻路函数传参)
// 当前位置落脚点到下一个位置的落脚点的高度如果下降了，那么最高只能下降65535米(这个值可在寻路函数传参)
// 当前位置落脚点到下一个位置的落脚点时，下一个落脚点的空隙最高点-当前位置落脚点要大于物体的高度(物体的高度这个值可在寻路函数传参)
// 当前位置落脚点到下一个位置的落脚点时，某些落脚点的Texture禁止落脚(比如水, 哪些落脚点的Texture不能落脚可在寻路函数传参)
// 计算下一个位置的落脚点的空隙时，可以选择忽略指定类型的Texture阻挡(寻路函数传参这个Texture)，只能忽略高于当前落脚点的部分，这部分认为是一段空隙。

// 新Html要求：
// 新html是3d版本的mra_visualizer.html。复刻golang版本的数据结构以及相关函数以及给你的html文件中的mra* 2d算法，来实现上面要求的mra* 3d寻路算法。
// 由于算法和地图数据都是3d的了，所以页面的可视化表现形式需要大改，完全弃用给你的html文件的表现形式，需要能以3d的形式表现。
// 页面能够非常方便的设置阻挡、地形、起点、终点，可以很直观的看到寻路过程及结果，可以以3d的模式去看。
// 页面可以设置网格x,z的大小（单位m，对应格子数量就要乘以16,1x1米有16格），x，z必须是32的倍数。
// 页面有设置地形模式，地形高度一定是从0开始，并且地形必定是低精的。
// 页面有设置阻挡模式，设置阻挡模式有细分高精还是低精模式，使用低精模式是直接设置1x1内16个格子的阻挡，高精模式则设置的是高精格的阻挡 （0.25mx0.25m）
// 阻挡有擦除模式，有办法比较容易的擦除阻挡。
// 页面可以设置w1 (WA*)、w2 (gate)、速度、实现给你html文件里的重置、清空障碍、单步、开始、暂停、跑到结束按钮功能
// 页面可以指定寻路函数里所有的传参
