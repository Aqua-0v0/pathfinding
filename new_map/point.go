package zmap3base

// Point2d 表示二维世界的唯一一点.
type Point2d struct {
	X       uint16
	Y       uint16
	XOffset uint8 // x 偏移值
	YOffset uint8 // y 偏移值
}

// LowPrecision 是否是低精度点（1x1的tile）
func (p Point2d) LowPrecision() bool {
	return p.XOffset == 0 // XOffset与YOffset都为0，即是低精点，由于XOffset为0时YOffset必为0，所以只判断一个
}

// LowPrecisionPoint 返回低精度点
func (p Point2d) LowPrecisionPoint() Point2d {
	return Point2d{X: p.X, Y: p.Y}
}

func (p Point2d) LoopHighPrecisionPointExt(f func(d Point2d) bool) {
	for x := uint8(1); x <= SecondaryAccuracy; x++ {
		for y := uint8(1); y <= SecondaryAccuracy; y++ {
			if !f(Point2d{X: p.X, Y: p.Y, XOffset: x, YOffset: y}) {
				return
			}
		}
	}
}

// WrapHighPrecisionPoint2d 将坐标 x, y, h 包装为 Point2d. x, y 的范围检查要在外部判断.
func WrapHighPrecisionPoint2d(xFloat, yFloat float32) Point2d {
	x, y := uint16(xFloat), uint16(yFloat)
	xFloat -= float32(x)
	yFloat -= float32(y)
	return Point2d{X: x, Y: y, XOffset: uint8((xFloat / SecondaryTileLen) + 1), YOffset: uint8((yFloat / SecondaryTileLen) + 1)}
}

type Point3d struct {
	X        uint16 // x 整点坐标
	Y        uint16 // y 整点坐标
	XOffset  uint8  // x 偏移值
	YOffset  uint8  // y 偏移值
	H        uint16 // Range.Begin
	RangeEnd uint16 // Range.End
}

// Point2d 返回 Point3d 对应的 Point2d.
func (p Point3d) Point2d() Point2d {
	return Point2d{X: p.X, Y: p.Y, XOffset: p.XOffset, YOffset: p.YOffset}
}
