package zmap3base

import (
	"fmt"
)

type Rect struct {
	// Min 表示矩形的左下角边界, inclusive; Max 表示矩形的右上角边界, exclusive.
	Min, Max Point2d
}

func (rect Rect) Width() uint16 {
	return rect.Max.X - rect.Min.X
}

func (rect Rect) Height() uint16 {
	return rect.Max.Y - rect.Min.Y
}

// AreaSize 虽然 uint64 量纲上不对, areaSize 类型范围应当尽可能大.
func (rect Rect) AreaSize() uint64 {
	return uint64(rect.Width()) * uint64(rect.Height())
}

// ContainsPoint 判断 rect 是否包含点 p（Min inclusive, Max exclusive）。
func (rect Rect) ContainsPoint(p Point2d) bool {
	return rect.Min.X <= p.X && p.X < rect.Max.X &&
		rect.Min.Y <= p.Y && p.Y < rect.Max.Y
}

// Contains 判断 rect 是否包含 other.
func (rect Rect) Contains(other Rect) bool {
	return rect.Min.X <= other.Min.X && rect.Min.Y <= other.Min.Y &&
		rect.Max.X >= other.Max.X && rect.Max.Y >= other.Max.Y
}

func (rect Rect) String() string {
	return fmt.Sprintf("[%v, %v)", rect.Min, rect.Max)
}
