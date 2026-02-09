package main

// RichRange 阻挡高度范围信息及阻挡材质和配置信息
type RichRange struct {
	Range
	Accessory
}

// Texture 表示区间的材质类型及附加属性.
type Texture uint32

type Accessory struct {
	// Texture 区间的材质信息
	Texture Texture
	// Config 交互物配置.
	Config uint32
}

// Range 体素阻挡的高度区间[Begin,End)，转化成真实世界的高度需要进行转化[float32(Begin)/20, float32(End)/20)
type Range struct {
	Begin uint16
	End   uint16
}

// RichRangeSetData 某个体素x,z位置的所有阻挡高度范围
type RichRangeSetData struct {
	raw []RichRange // 数据里的数据可以是按照某种顺序有序地，这个你来定
}
