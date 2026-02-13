package zmap3base

// FastGridSetSize 地形分块网格长度.
const FastGridSetSize = 32 // 必须为2的n次幂
const FastGridCellNum = FastGridSetSize * FastGridSetSize

const MaxRangeEnd = 0x_FF00

// 如果网格是高精度的网格时：
const SecondaryAccuracy = 4                                    // 1x1的x,z网格的单边被分成了4等份
const SecondaryTileNum = SecondaryAccuracy * SecondaryAccuracy // 1x1的x,z网格被分成了16等份
const SecondaryTileLen = 1 / float32(SecondaryAccuracy)        // 二级tile的边长
