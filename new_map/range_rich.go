package zmap3base

import (
	"fmt"
	"math"
)

type Climate uint16

// Range 高度区间
type Range struct {
	Begin uint16 // 映射到真实世界高度需要除以20，所以高度的精度为0.05
	End   uint16 // 映射到真实世界高度需要除以20，所以高度的精度为0.05
}

var MaxRange = Range{0, MaxRangeEnd}

// Contains 判断 h 是否在范围内.
func (rg Range) Contains(h uint16) bool {
	return rg.Begin <= h && h < rg.End
}

func (rg Range) Valid() bool {
	return rg.Begin <= rg.End
}

// Len 返回范围的长度.
func (rg Range) Len() uint16 {
	return rg.End - rg.Begin
}

// Overlap 判断与 other 是否有重合范围.
func (rg Range) Overlap(other Range) bool {
	low := rg.Begin
	if other.Begin > low {
		low = other.Begin
	}
	high := rg.End
	if other.End < high {
		high = other.End
	}
	return low < high
}

// Intersect 返回两个范围的交集.
func (rg Range) Intersect(other Range) Range {
	if rg.End < other.Begin {
		return Range{}
	}
	if other.End < rg.Begin {
		return Range{}
	}
	return Range{max(rg.Begin, other.Begin), min(rg.End, other.End)}
}

func WrapRange(begin, end uint16) Range {
	return Range{begin, end}
}

type Accessory struct {
	// Texture 区间的附加信息, 仅供服务器内部使用.
	Texture Texture
	// Config 交互物配置.
	Config uint32
}

// IntoUint64 将 Accessory 转换为 uint64.
func (acc Accessory) IntoUint64() (u64 uint64) {
	return (uint64(acc.Texture) << 32) | uint64(acc.Config)
}

// FromUint64 将 uint64 转换为 Accessory.
func (acc *Accessory) FromUint64(u64 uint64) {
	acc.Texture = Texture(u64 >> 32)
	acc.Config = uint32(u64)
}

func (acc Accessory) String() string {
	return fmt.Sprintf("Config: %d, Texture: %v", acc.Config, acc.Texture)
}

type Texture uint32

const (
	txtOffsetMaterial  = 0
	txtOffsetProperty  = txtOffsetMaterial + 8
	txtOffsetBlockType = txtOffsetProperty + 12
	txtOffsetClassType = txtOffsetBlockType + 9
)

// Texture Material 类型.
const (
	// TextureMaterBase 表示地表材质, 大地图表面所属材质类型. 水体包含在内.
	TextureMaterBase Texture = 1 << (iota + txtOffsetMaterial)
	// TextureMaterVoxel 表示体素材质.
	TextureMaterVoxel
	// TextureMaterObstacle 表示粗精度阻挡.
	TextureMaterObstacle
	// TextureMaterCollider 表示精细化碰撞.
	TextureMaterCollider

	// TextureMaterVehicle 表示载具的材质类型, TextureMaterCollider 特化.
	TextureMaterVehicle
	// TextureMaterMonster 表示 Monster 类型, TextureMaterCollider 特化.
	TextureMaterMonster

	textureMaterLimit
)

// Texture Property 类型.
const (
	// TexturePropWater 表示水体.
	TexturePropWater Texture = 1 << (iota + txtOffsetProperty)
	// TexturePropSolid 阻挡是否为实心阻挡.
	TexturePropSolid
	// TexturePropWaterDeep 水体但位于深水区.
	TexturePropWaterDeep
	// TexturePropHuman 人形
	TexturePropHuman

	texturePropLimit
)

// Texture BlockType 类型.
const (
	// TextureBlcTypNone 表示不包含任何 BlockType.
	TextureBlcTypNone Texture = 1 << (txtOffsetClassType - 1)
)

// Texture Mask 定义.
const (
	// TextureMaskEverything 所有 bit 置位.
	TextureMaskEverything Texture = math.MaxUint32
	// TextureMaskEveryMaterial 所有 Material 置位.
	TextureMaskEveryMaterial Texture = 1<<txtOffsetProperty - 1
	// TextureMaskEveryProperty 所有 Property 置位.
	TextureMaskEveryProperty Texture = (1<<txtOffsetBlockType - 1) &^ (1<<txtOffsetProperty - 1)
	// TextureMaskEveryBlockType 所有 BlockType 置位.
	TextureMaskEveryBlockType Texture = (1<<txtOffsetClassType - 1) &^ (1<<txtOffsetBlockType - 1)
	// TextureMaskEveryClassType 所有 ClassType 置位.
	TextureMaskEveryClassType Texture = TextureMaskEverything &^ (1<<txtOffsetClassType - 1)

	// TextureMaskGeneralBase 广义的基础地形 (base, 体素).
	TextureMaskGeneralBase Texture = TextureMaterBase | TextureMaterVoxel
	// TextureMaskGeneralCollider 广义的 Collider.
	TextureMaskGeneralCollider Texture = TextureMaterCollider | TextureMaterVehicle | TextureMaterMonster
	// TextureMaskGeneralWater 广义的水体.
	TextureMaskGeneralWater = TexturePropWater | TexturePropWaterDeep
)

// ---------------RichRange 携带更多信息.-----------------

type RichRange struct {
	Range
	Accessory
}

func (rrg RichRange) getTexture() Texture {
	return rrg.Accessory.Texture
}

func (rrg RichRange) String() string {
	return fmt.Sprintf("RichRange{Range: %v, %v}", rrg.Range, rrg.Accessory)
}

// --------------SnapRichRange 本质为多个 RichRange 叠成的快照.-----------------

type SnapRichRange struct {
	Range
	Texture Texture
}
