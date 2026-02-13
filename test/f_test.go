package hpc

import (
	"math"
	"pathfinding/map_data"
	"runtime"
	"testing"
	"unsafe"
)

type Same [5]uint16

// DumpSameUltra32_LOHI:
// 返回 32个 uint16：f0,n0,f1,n1,...f15,n15
// 每组读取: 1bit flag + 4bit nibble
// - 无 for
// - 无 helper
// - 无分支
func (s Same) DumpSameUltra32_LOHI() (
	f0, n0, f1, n1, f2, n2, f3, n3,
	f4, n4, f5, n5, f6, n6, f7, n7,
	f8, n8, f9, n9, f10, n10, f11, n11,
	f12, n12, f13, n13, f14, n14, f15, n15 uint16,
) {
	lo := uint64(s[0]) |
		(uint64(s[1]) << 16) |
		(uint64(s[2]) << 32) |
		(uint64(s[3]) << 48)
	hi := uint64(s[4])

	// group0 start0 => bits0..4
	x0 := uint16((lo >> 0) & 0x1F)
	f0 = x0 & 1
	n0 = (x0 >> 1) & 0x0F

	// group1 start5
	x1 := uint16((lo >> 5) & 0x1F)
	f1 = x1 & 1
	n1 = (x1 >> 1) & 0x0F

	// group2 start10
	x2 := uint16((lo >> 10) & 0x1F)
	f2 = x2 & 1
	n2 = (x2 >> 1) & 0x0F

	// group3 start15
	x3 := uint16((lo >> 15) & 0x1F)
	f3 = x3 & 1
	n3 = (x3 >> 1) & 0x0F

	// group4 start20
	x4 := uint16((lo >> 20) & 0x1F)
	f4 = x4 & 1
	n4 = (x4 >> 1) & 0x0F

	// group5 start25
	x5 := uint16((lo >> 25) & 0x1F)
	f5 = x5 & 1
	n5 = (x5 >> 1) & 0x0F

	// group6 start30
	x6 := uint16((lo >> 30) & 0x1F)
	f6 = x6 & 1
	n6 = (x6 >> 1) & 0x0F

	// group7 start35
	x7 := uint16((lo >> 35) & 0x1F)
	f7 = x7 & 1
	n7 = (x7 >> 1) & 0x0F

	// group8 start40
	x8 := uint16((lo >> 40) & 0x1F)
	f8 = x8 & 1
	n8 = (x8 >> 1) & 0x0F

	// group9 start45
	x9 := uint16((lo >> 45) & 0x1F)
	f9 = x9 & 1
	n9 = (x9 >> 1) & 0x0F

	// group10 start50
	x10 := uint16((lo >> 50) & 0x1F)
	f10 = x10 & 1
	n10 = (x10 >> 1) & 0x0F

	// group11 start55
	x11 := uint16((lo >> 55) & 0x1F)
	f11 = x11 & 1
	n11 = (x11 >> 1) & 0x0F

	// group12 start60
	x12 := uint16((lo >> 60) & 0x1F)
	f12 = x12 & 1
	n12 = (x12 >> 1) & 0x0F

	// group13 start65 => hi bit1..5
	x13 := uint16((hi >> 1) & 0x1F)
	f13 = x13 & 1
	n13 = (x13 >> 1) & 0x0F

	// group14 start70 => hi bit6..10
	x14 := uint16((hi >> 6) & 0x1F)
	f14 = x14 & 1
	n14 = (x14 >> 1) & 0x0F

	// group15 start75 => hi bit11..15
	x15 := uint16((hi >> 11) & 0x1F)
	f15 = x15 & 1
	n15 = (x15 >> 1) & 0x0F

	return
}

func (s Same) DumpSameUltra32_Raw() (
	f0, n0, f1, n1, f2, n2, f3, n3,
	f4, n4, f5, n5, f6, n6, f7, n7,
	f8, n8, f9, n9, f10, n10, f11, n11,
	f12, n12, f13, n13, f14, n14, f15, n15 uint16,
) {
	w0 := uint32(s[0])
	w1 := uint32(s[1])
	w2 := uint32(s[2])
	w3 := uint32(s[3])
	w4 := uint32(s[4])

	// group0 start0 word0 shift0
	x0 := uint16((w0 >> 0) & 0x1F)
	f0 = x0 & 1
	n0 = (x0 >> 1) & 0x0F

	// group1 start5 word0 shift5
	x1 := uint16((w0 >> 5) & 0x1F)
	f1 = x1 & 1
	n1 = (x1 >> 1) & 0x0F

	// group2 start10 word0 shift10
	x2 := uint16((w0 >> 10) & 0x1F)
	f2 = x2 & 1
	n2 = (x2 >> 1) & 0x0F

	// group3 start15 cross w0->w1
	x3 := uint16(((w0 >> 15) | (w1 << 1)) & 0x1F)
	f3 = x3 & 1
	n3 = (x3 >> 1) & 0x0F

	// group4 start20 word1 shift4
	x4 := uint16((w1 >> 4) & 0x1F)
	f4 = x4 & 1
	n4 = (x4 >> 1) & 0x0F

	// group5 start25 word1 shift9
	x5 := uint16((w1 >> 9) & 0x1F)
	f5 = x5 & 1
	n5 = (x5 >> 1) & 0x0F

	// group6 start30 cross w1->w2
	x6 := uint16(((w1 >> 14) | (w2 << 2)) & 0x1F)
	f6 = x6 & 1
	n6 = (x6 >> 1) & 0x0F

	// group7 start35 word2 shift3
	x7 := uint16((w2 >> 3) & 0x1F)
	f7 = x7 & 1
	n7 = (x7 >> 1) & 0x0F

	// group8 start40 word2 shift8
	x8 := uint16((w2 >> 8) & 0x1F)
	f8 = x8 & 1
	n8 = (x8 >> 1) & 0x0F

	// group9 start45 cross w2->w3
	x9 := uint16(((w2 >> 13) | (w3 << 3)) & 0x1F)
	f9 = x9 & 1
	n9 = (x9 >> 1) & 0x0F

	// group10 start50 word3 shift2
	x10 := uint16((w3 >> 2) & 0x1F)
	f10 = x10 & 1
	n10 = (x10 >> 1) & 0x0F

	// group11 start55 word3 shift7
	x11 := uint16((w3 >> 7) & 0x1F)
	f11 = x11 & 1
	n11 = (x11 >> 1) & 0x0F

	// group12 start60 cross w3->w4
	x12 := uint16(((w3 >> 12) | (w4 << 4)) & 0x1F)
	f12 = x12 & 1
	n12 = (x12 >> 1) & 0x0F

	// group13 start65 word4 shift1
	x13 := uint16((w4 >> 1) & 0x1F)
	f13 = x13 & 1
	n13 = (x13 >> 1) & 0x0F

	// group14 start70 word4 shift6
	x14 := uint16((w4 >> 6) & 0x1F)
	f14 = x14 & 1
	n14 = (x14 >> 1) & 0x0F

	// group15 start75 word4 shift11
	x15 := uint16((w4 >> 11) & 0x1F)
	f15 = x15 & 1
	n15 = (x15 >> 1) & 0x0F

	return
}

func (s Same) DumpSameUltra32_LOHI_No5bitTmp() (
	f0, n0, f1, n1, f2, n2, f3, n3,
	f4, n4, f5, n5, f6, n6, f7, n7,
	f8, n8, f9, n9, f10, n10, f11, n11,
	f12, n12, f13, n13, f14, n14, f15, n15 uint16,
) {
	lo := uint64(s[0]) |
		(uint64(s[1]) << 16) |
		(uint64(s[2]) << 32) |
		(uint64(s[3]) << 48)
	hi := uint64(s[4])

	// group0 start=0: flag=bit0, nibble=bit1..4
	f0 = uint16((lo >> 0) & 1)
	n0 = uint16((lo >> 1) & 0x0F)

	// group1 start=5: flag=bit5, nibble=bit6..9
	f1 = uint16((lo >> 5) & 1)
	n1 = uint16((lo >> 6) & 0x0F)

	// group2 start=10: flag=bit10, nibble=bit11..14
	f2 = uint16((lo >> 10) & 1)
	n2 = uint16((lo >> 11) & 0x0F)

	// group3 start=15: flag=bit15, nibble=bit16..19
	f3 = uint16((lo >> 15) & 1)
	n3 = uint16((lo >> 16) & 0x0F)

	// group4 start=20: flag=bit20, nibble=bit21..24
	f4 = uint16((lo >> 20) & 1)
	n4 = uint16((lo >> 21) & 0x0F)

	// group5 start=25: flag=bit25, nibble=bit26..29
	f5 = uint16((lo >> 25) & 1)
	n5 = uint16((lo >> 26) & 0x0F)

	// group6 start=30: flag=bit30, nibble=bit31..34
	f6 = uint16((lo >> 30) & 1)
	n6 = uint16((lo >> 31) & 0x0F)

	// group7 start=35: flag=bit35, nibble=bit36..39
	f7 = uint16((lo >> 35) & 1)
	n7 = uint16((lo >> 36) & 0x0F)

	// group8 start=40: flag=bit40, nibble=bit41..44
	f8 = uint16((lo >> 40) & 1)
	n8 = uint16((lo >> 41) & 0x0F)

	// group9 start=45: flag=bit45, nibble=bit46..49
	f9 = uint16((lo >> 45) & 1)
	n9 = uint16((lo >> 46) & 0x0F)

	// group10 start=50: flag=bit50, nibble=bit51..54
	f10 = uint16((lo >> 50) & 1)
	n10 = uint16((lo >> 51) & 0x0F)

	// group11 start=55: flag=bit55, nibble=bit56..59
	f11 = uint16((lo >> 55) & 1)
	n11 = uint16((lo >> 56) & 0x0F)

	// group12 start=60: flag=bit60, nibble=bit61..64 (注意：这里 nibble 跨到 hi 的 bit0)
	// 直接用拼接 lo/hi 的方式处理：lo 只到 bit63，所以 bit64 来自 hi bit0
	// n12 = bits61..64 => lo>>61 取 3bit + hi bit0 作为最高 bit
	// 写成完全无临时变量的表达式：
	f12 = uint16((lo >> 60) & 1)
	n12 = uint16(((lo >> 61) & 0x07) | ((hi & 1) << 3))

	// group13 start=65: flag=bit65 (hi bit1), nibble=bit66..69 (hi bit2..5)
	f13 = uint16((hi >> 1) & 1)
	n13 = uint16((hi >> 2) & 0x0F)

	// group14 start=70: flag=bit70 (hi bit6), nibble=bit71..74 (hi bit7..10)
	f14 = uint16((hi >> 6) & 1)
	n14 = uint16((hi >> 7) & 0x0F)

	// group15 start=75: flag=bit75 (hi bit11), nibble=bit76..79 (hi bit12..15)
	f15 = uint16((hi >> 11) & 1)
	n15 = uint16((hi >> 12) & 0x0F)

	return
}

var sinkF0, sinkN0, sinkF1, sinkN1, sinkF2, sinkN2, sinkF3, sinkN3 uint16
var sinkF4, sinkN4, sinkF5, sinkN5, sinkF6, sinkN6, sinkF7, sinkN7 uint16
var sinkF8, sinkN8, sinkF9, sinkN9, sinkF10, sinkN10, sinkF11, sinkN11 uint16
var sinkF12, sinkN12, sinkF13, sinkN13, sinkF14, sinkN14, sinkF15, sinkN15 uint16

func BenchmarkDumpSameUltra32_LOHI(b *testing.B) {
	b.ReportAllocs()
	s := Same{0x1234, 0xABCD, 0x0F0F, 0xAAAA, 0x5555}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkF0, sinkN0, sinkF1, sinkN1, sinkF2, sinkN2, sinkF3, sinkN3,
			sinkF4, sinkN4, sinkF5, sinkN5, sinkF6, sinkN6, sinkF7, sinkN7,
			sinkF8, sinkN8, sinkF9, sinkN9, sinkF10, sinkN10, sinkF11, sinkN11,
			sinkF12, sinkN12, sinkF13, sinkN13, sinkF14, sinkN14, sinkF15, sinkN15 = s.DumpSameUltra32_LOHI()
	}
}

func BenchmarkDumpSameUltra32_Raw(b *testing.B) {
	b.ReportAllocs()
	s := Same{0x1234, 0xABCD, 0x0F0F, 0xAAAA, 0x5555}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkF0, sinkN0, sinkF1, sinkN1, sinkF2, sinkN2, sinkF3, sinkN3,
			sinkF4, sinkN4, sinkF5, sinkN5, sinkF6, sinkN6, sinkF7, sinkN7,
			sinkF8, sinkN8, sinkF9, sinkN9, sinkF10, sinkN10, sinkF11, sinkN11,
			sinkF12, sinkN12, sinkF13, sinkN13, sinkF14, sinkN14, sinkF15, sinkN15 = s.DumpSameUltra32_Raw()
	}
}

func BenchmarkDumpSameUltra32_LOHI_No5bitTmp(b *testing.B) {
	b.ReportAllocs()
	s := Same{0x1234, 0xABCD, 0x0F0F, 0xAAAA, 0x5555}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.DumpSameUltra32_LOHI_No5bitTmp()
	}
}

// setBit：在 Same 80-bit bitstream 上设置某个 bit（bit0 是 s[0] 的最低位）
// 测试代码允许 for / helper，不影响生产代码性能
func setBit(s *Same, bitPos int, v uint16) {
	word := bitPos >> 4  // /16
	shift := bitPos & 15 // %16
	mask := uint16(1 << shift)
	if v != 0 {
		(*s)[word] |= mask
	} else {
		(*s)[word] &^= mask
	}
}

// writeGroup：写入第 idx 组（0..15），格式是：1bit flag + 4bit nibble（低位在前）
// groupStart = idx * 5
// bit order: start+0 = flag
//
//	start+1..start+4 = nibble bit0..bit3
func writeGroup(s *Same, idx int, flag uint16, nibble uint16) {
	start := idx * 5
	setBit(s, start+0, flag&1)
	setBit(s, start+1, (nibble>>0)&1)
	setBit(s, start+2, (nibble>>1)&1)
	setBit(s, start+3, (nibble>>2)&1)
	setBit(s, start+4, (nibble>>3)&1)
}

func TestDumpSameUltra32_LOHI_No5bitTmp(t *testing.T) {
	var s Same

	// 构造 16 组测试数据：
	// flag = idx % 2
	// nibble = (idx*3 + 5) & 0xF  (选个非线性模式，确保不是简单递增，容易覆盖跨界情况)
	for i := 0; i < 16; i++ {
		flag := uint16(i & 1)
		nibble := uint16((i*3 + 5) & 0xF)
		writeGroup(&s, i, flag, nibble)
	}

	// 调用待测函数
	f0, n0, f1, n1, f2, n2, f3, n3,
		f4, n4, f5, n5, f6, n6, f7, n7,
		f8, n8, f9, n9, f10, n10, f11, n11,
		f12, n12, f13, n13, f14, n14, f15, n15 := s.DumpSameUltra32_LOHI_No5bitTmp()

	// 期望值（和上面 writeGroup 的模式一致）
	wantF := [16]uint16{
		0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 0, 1, 0, 1,
	}
	wantN := [16]uint16{
		5, 8, 11, 14, 1, 4, 7, 10,
		13, 0, 3, 6, 9, 12, 15, 2,
	}

	// 对比（展开写，确保每组都严格验证）
	if f0 != wantF[0] || n0 != wantN[0] {
		t.Fatalf("g0 mismatch got (f=%d n=%d) want (f=%d n=%d)", f0, n0, wantF[0], wantN[0])
	}
	if f1 != wantF[1] || n1 != wantN[1] {
		t.Fatalf("g1 mismatch got (f=%d n=%d) want (f=%d n=%d)", f1, n1, wantF[1], wantN[1])
	}
	if f2 != wantF[2] || n2 != wantN[2] {
		t.Fatalf("g2 mismatch got (f=%d n=%d) want (f=%d n=%d)", f2, n2, wantF[2], wantN[2])
	}
	if f3 != wantF[3] || n3 != wantN[3] {
		t.Fatalf("g3 mismatch got (f=%d n=%d) want (f=%d n=%d)", f3, n3, wantF[3], wantN[3])
	}
	if f4 != wantF[4] || n4 != wantN[4] {
		t.Fatalf("g4 mismatch got (f=%d n=%d) want (f=%d n=%d)", f4, n4, wantF[4], wantN[4])
	}
	if f5 != wantF[5] || n5 != wantN[5] {
		t.Fatalf("g5 mismatch got (f=%d n=%d) want (f=%d n=%d)", f5, n5, wantF[5], wantN[5])
	}
	if f6 != wantF[6] || n6 != wantN[6] {
		t.Fatalf("g6 mismatch got (f=%d n=%d) want (f=%d n=%d)", f6, n6, wantF[6], wantN[6])
	}
	if f7 != wantF[7] || n7 != wantN[7] {
		t.Fatalf("g7 mismatch got (f=%d n=%d) want (f=%d n=%d)", f7, n7, wantF[7], wantN[7])
	}
	if f8 != wantF[8] || n8 != wantN[8] {
		t.Fatalf("g8 mismatch got (f=%d n=%d) want (f=%d n=%d)", f8, n8, wantF[8], wantN[8])
	}
	if f9 != wantF[9] || n9 != wantN[9] {
		t.Fatalf("g9 mismatch got (f=%d n=%d) want (f=%d n=%d)", f9, n9, wantF[9], wantN[9])
	}
	if f10 != wantF[10] || n10 != wantN[10] {
		t.Fatalf("g10 mismatch got (f=%d n=%d) want (f=%d n=%d)", f10, n10, wantF[10], wantN[10])
	}
	if f11 != wantF[11] || n11 != wantN[11] {
		t.Fatalf("g11 mismatch got (f=%d n=%d) want (f=%d n=%d)", f11, n11, wantF[11], wantN[11])
	}

	// ✅ group12 特别关键：n12 跨界 (lo bit61..63 + hi bit0)
	if f12 != wantF[12] || n12 != wantN[12] {
		t.Fatalf("g12 mismatch got (f=%d n=%d) want (f=%d n=%d)", f12, n12, wantF[12], wantN[12])
	}

	if f13 != wantF[13] || n13 != wantN[13] {
		t.Fatalf("g13 mismatch got (f=%d n=%d) want (f=%d n=%d)", f13, n13, wantF[13], wantN[13])
	}
	if f14 != wantF[14] || n14 != wantN[14] {
		t.Fatalf("g14 mismatch got (f=%d n=%d) want (f=%d n=%d)", f14, n14, wantF[14], wantN[14])
	}
	if f15 != wantF[15] || n15 != wantN[15] {
		t.Fatalf("g15 mismatch got (f=%d n=%d) want (f=%d n=%d)", f15, n15, wantF[15], wantN[15])
	}
}

// ---- 用于防止编译器优化掉结果 ----
var sinkSpans []map_data.Span
var sinkSpan map_data.Span

// ---- 构造一个 HighPrecisionColumn，支持可控的 Same 编码 ----
//
// 我们需要做到：把 16 组 (flag,nibble) 写入 Same 的 80-bit 流。
// 格式：每组 5bit：bit0=flag, bit1..4=nibble (little-endian)
//
// sliceIndex 的语义：
// - group0 不更新 sliceIndex
// - group1..15 每组如果 flag=1，则 sliceIndex++
// - 如果 grid==p.GridIndex，返回 spans[sliceIndex]
//
// 所以要让命中 groupK 对应 spans[idxK]：
// 你需要合理设置 flag 前缀和。
func makeColumnForHitGroup(hitGroup int) *map_data.Column {
	var same map_data.Same

	// flags：让 sliceIndex 每组都递增（flag=1），这样 sliceIndex == groupIndex
	// 但注意 group0 不累加 idx，所以为了让 group1 命中 spans[1]，group1 flag=1 是必须的
	// 最简单：flag[1..15]=1，flag0随便
	// 这样 idx 在 group1 检查前先+1，group2前再+1...
	flags := [16]uint16{
		0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	}

	// grids：每组 nibble 设置成 groupIndex（0..15）
	// 这样只要 p.GridIndex = hitGroup，就会命中对应 group
	grids := [16]uint16{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	}

	// 将 16 组写入 same bitstream
	for i := 0; i < 16; i++ {
		writeGroup1(&same, i, flags[i], grids[i])
	}

	// spans：至少 16 个
	spans := make([][]map_data.Span, 16)
	for i := 0; i < 16; i++ {
		spans[i] = []map_data.Span{{}}
	}
	return &map_data.Column{
		Terrain: map_data.Span{},
		HighPrecision: &map_data.HighPrecisionColumn{
			Same:  same,
			Spans: spans,
		},
	}
}

// writeGroup：写入第 idx 组（0..15），格式是：1bit flag + 4bit nibble（低位在前）
func writeGroup1(s *map_data.Same, idx int, flag uint16, nibble uint16) {
	start := idx * 5
	setBit1(s, start+0, flag&1)
	setBit1(s, start+1, (nibble>>0)&1)
	setBit1(s, start+2, (nibble>>1)&1)
	setBit1(s, start+3, (nibble>>2)&1)
	setBit1(s, start+4, (nibble>>3)&1)
}

func setBit1(s *map_data.Same, bitPos int, v uint16) {
	word := bitPos >> 4
	shift := bitPos & 15
	mask := uint16(1 << shift)
	if v != 0 {
		(*s)[word] |= mask
	} else {
		(*s)[word] &^= mask
	}
}

func BenchmarkGetHighPrecisionOtherSpan(b *testing.B) {
	// 三种情况：命中最早、命中最晚、不命中
	colHit0 := makeColumnForHitGroup(0)
	colHit15 := makeColumnForHitGroup(15)
	colMiss := makeColumnForHitGroup(0) // 用同一个same，但 p.GridIndex 设成 99 => miss

	pHit0 := map_data.PointForSearchPath{GridIndex: 0}
	pHit15 := map_data.PointForSearchPath{GridIndex: 15}
	pMiss := map_data.PointForSearchPath{GridIndex: 99}

	// -------- Fast 版本 --------
	b.Run("Fast/Hit0", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkSpan, sinkSpans = colHit0.GetSpans(pHit0)
		}
		runtime.KeepAlive(colHit0)
	})

	b.Run("Fast/Hit15", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkSpan, sinkSpans = colHit15.GetSpans(pHit15)
		}
		runtime.KeepAlive(colHit15)
	})

	b.Run("Fast/Miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkSpan, sinkSpans = colMiss.GetSpans(pMiss)
		}
		runtime.KeepAlive(colMiss)
	})
}

// Span 阻挡阻挡区间数据及阻挡Texture信息
type Span struct {
	MinY    uint16
	MaxY    uint16
	Texture uint32
}

// GetInterval 返回相对于当前位置，这个柱列阻挡的可通行的空隙
func GetInterval1(
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

	gapMinY := terrain.MaxY
	gapTexture := terrain.Texture

	a := ignoreTexture != 0

	for _, v := range spans {
		// ignoreTexture：跳过这段阻挡
		if a && (v.Texture&ignoreTexture) != 0 {
			continue
		}

		// v 在 gapMinY 下方：无影响
		if v.MaxY <= gapMinY {
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

// GetInterval 返回相对于当前位置，这个柱列阻挡的可通行的空隙
func GetInterval(
	terrain Span,
	spans []Span,
	curY int32,
	ignoreTexture, forbiddenTexture uint32,
	height, upLimit, downLimit int32,
) (result Span, ok bool) {

	n := len(spans)
	if n == 0 {
		if forbiddenTexture != 0 && (terrain.Texture&forbiddenTexture) != 0 {
			return
		}
		tMax := int32(terrain.MaxY)
		if tMax > curY+upLimit {
			return
		}
		if tMax < curY-downLimit {
			return
		}
		return terrain, true
	}

	needTopY := curY + height
	minAllowed := curY - downLimit
	maxAllowed := curY + upLimit

	// height 在你当前语义里始终被当作 uint16 使用（你原代码也是 uint16(height)）
	h16 := uint16(height)

	gapMinY := terrain.MaxY
	gapTex := terrain.Texture

	// -------- 三路特化：把分支从循环中挪走 --------
	if ignoreTexture == 0 {
		if forbiddenTexture == 0 {
			return getInterval_U8_I0F0(spans, n, gapMinY, gapTex, needTopY, minAllowed, maxAllowed, h16)
		}
		return getInterval_U8_I0F1(spans, n, gapMinY, gapTex, forbiddenTexture, needTopY, minAllowed, maxAllowed, h16)
	}

	return getInterval_U8_General(spans, n, gapMinY, gapTex, ignoreTexture, forbiddenTexture, needTopY, minAllowed, maxAllowed, h16)
}

// ------------------- ignore=0 forbidden=0 极热路径 -------------------
func getInterval_U8_I0F0(
	spans []Span,
	n int,
	gapMinY uint16,
	gapTex uint32,
	needTopY int32,
	minAllowed, maxAllowed int32,
	h16 uint16,
) (result Span, ok bool) {

	// unsafe 指针遍历：减少 bounds check + index 计算
	p := unsafe.Pointer(&spans[0])
	end := unsafe.Pointer(uintptr(p) + uintptr(n)*unsafe.Sizeof(Span{}))

	// unroll×8
	for uintptr(p)+8*unsafe.Sizeof(Span{}) <= uintptr(end) {

		// ----- 0 -----
		v0 := *(*Span)(p)
		if v0.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v0.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v0.MaxY
			gapTex = v0.Texture
		}

		// ----- 1 -----
		v1 := *(*Span)(unsafe.Add(p, 1*unsafe.Sizeof(Span{})))
		if v1.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v1.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v1.MaxY
			gapTex = v1.Texture
		}

		// ----- 2 -----
		v2 := *(*Span)(unsafe.Add(p, 2*unsafe.Sizeof(Span{})))
		if v2.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v2.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v2.MaxY
			gapTex = v2.Texture
		}

		// ----- 3 -----
		v3 := *(*Span)(unsafe.Add(p, 3*unsafe.Sizeof(Span{})))
		if v3.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v3.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v3.MaxY
			gapTex = v3.Texture
		}

		// ----- 4 -----
		v4 := *(*Span)(unsafe.Add(p, 4*unsafe.Sizeof(Span{})))
		if v4.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v4.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v4.MaxY
			gapTex = v4.Texture
		}

		// ----- 5 -----
		v5 := *(*Span)(unsafe.Add(p, 5*unsafe.Sizeof(Span{})))
		if v5.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v5.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v5.MaxY
			gapTex = v5.Texture
		}

		// ----- 6 -----
		v6 := *(*Span)(unsafe.Add(p, 6*unsafe.Sizeof(Span{})))
		if v6.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v6.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v6.MaxY
			gapTex = v6.Texture
		}

		// ----- 7 -----
		v7 := *(*Span)(unsafe.Add(p, 7*unsafe.Sizeof(Span{})))
		if v7.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v7.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					result.MinY = gapMinY
					result.MaxY = gapMaxY
					result.Texture = gapTex
					return result, true
				}
			}
			gapMinY = v7.MaxY
			gapTex = v7.Texture
		}

		// 前进 8 个
		p = unsafe.Add(p, 8*unsafe.Sizeof(Span{}))
	}

	// tail（剩余 0~7 个）
	for uintptr(p) < uintptr(end) {
		v := *(*Span)(p)
		if v.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v.MaxY
			gapTex = v.Texture
		}
		p = unsafe.Add(p, unsafe.Sizeof(Span{}))
	}

FINAL:
	// final gap: [gapMinY, MaxUint16)
	const finalMaxU16 = uint16(0xFFFF)
	finalMax := int32(finalMaxU16)

	if finalMax-int32(gapMinY) < int32(h16) {
		return
	}
	if finalMax < needTopY {
		return
	}
	gMinI := int32(gapMinY)
	if gMinI > maxAllowed || gMinI < minAllowed {
		return
	}

	return Span{gapMinY, finalMaxU16, gapTex}, true
}

// ------------------- ignore=0 forbidden!=0 -------------------
func getInterval_U8_I0F1(
	spans []Span,
	n int,
	gapMinY uint16,
	gapTex uint32,
	forbiddenTexture uint32,
	needTopY int32,
	minAllowed, maxAllowed int32,
	h16 uint16,
) (result Span, ok bool) {

	p := unsafe.Pointer(&spans[0])
	end := unsafe.Pointer(uintptr(p) + uintptr(n)*unsafe.Sizeof(Span{}))

	for uintptr(p)+8*unsafe.Sizeof(Span{}) <= uintptr(end) {

		v0 := *(*Span)(p)
		if v0.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v0.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v0.MaxY
			gapTex = v0.Texture
		}

		v1 := *(*Span)(unsafe.Add(p, 1*unsafe.Sizeof(Span{})))
		if v1.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v1.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v1.MaxY
			gapTex = v1.Texture
		}

		v2 := *(*Span)(unsafe.Add(p, 2*unsafe.Sizeof(Span{})))
		if v2.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v2.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v2.MaxY
			gapTex = v2.Texture
		}

		v3 := *(*Span)(unsafe.Add(p, 3*unsafe.Sizeof(Span{})))
		if v3.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v3.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v3.MaxY
			gapTex = v3.Texture
		}

		v4 := *(*Span)(unsafe.Add(p, 4*unsafe.Sizeof(Span{})))
		if v4.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v4.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v4.MaxY
			gapTex = v4.Texture
		}

		v5 := *(*Span)(unsafe.Add(p, 5*unsafe.Sizeof(Span{})))
		if v5.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v5.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v5.MaxY
			gapTex = v5.Texture
		}

		v6 := *(*Span)(unsafe.Add(p, 6*unsafe.Sizeof(Span{})))
		if v6.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v6.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v6.MaxY
			gapTex = v6.Texture
		}

		v7 := *(*Span)(unsafe.Add(p, 7*unsafe.Sizeof(Span{})))
		if v7.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v7.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v7.MaxY
			gapTex = v7.Texture
		}

		p = unsafe.Add(p, 8*unsafe.Sizeof(Span{}))
	}

	for uintptr(p) < uintptr(end) {
		v := *(*Span)(p)
		if v.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY && (gapTex&forbiddenTexture) == 0 {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v.MaxY
			gapTex = v.Texture
		}
		p = unsafe.Add(p, unsafe.Sizeof(Span{}))
	}

FINAL:
	const finalMaxU16 = uint16(0xFFFF)
	finalMax := int32(finalMaxU16)

	if finalMax-int32(gapMinY) < int32(h16) {
		return
	}
	if finalMax < needTopY {
		return
	}
	if (gapTex & forbiddenTexture) != 0 {
		return
	}
	gMinI := int32(gapMinY)
	if gMinI > maxAllowed || gMinI < minAllowed {
		return
	}

	return Span{gapMinY, finalMaxU16, gapTex}, true
}

// ------------------- general（ignore!=0） -------------------
func getInterval_U8_General(
	spans []Span,
	n int,
	gapMinY uint16,
	gapTex uint32,
	ignoreTexture, forbiddenTexture uint32,
	needTopY int32,
	minAllowed, maxAllowed int32,
	h16 uint16,
) (result Span, ok bool) {

	p := unsafe.Pointer(&spans[0])
	end := unsafe.Pointer(uintptr(p) + uintptr(n)*unsafe.Sizeof(Span{}))

	for uintptr(p)+8*unsafe.Sizeof(Span{}) <= uintptr(end) {

		v0 := *(*Span)(p)
		if (v0.Texture&ignoreTexture) == 0 && v0.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v0.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v0.MaxY
			gapTex = v0.Texture
		}

		v1 := *(*Span)(unsafe.Add(p, 1*unsafe.Sizeof(Span{})))
		if (v1.Texture&ignoreTexture) == 0 && v1.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v1.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v1.MaxY
			gapTex = v1.Texture
		}

		v2 := *(*Span)(unsafe.Add(p, 2*unsafe.Sizeof(Span{})))
		if (v2.Texture&ignoreTexture) == 0 && v2.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v2.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v2.MaxY
			gapTex = v2.Texture
		}

		v3 := *(*Span)(unsafe.Add(p, 3*unsafe.Sizeof(Span{})))
		if (v3.Texture&ignoreTexture) == 0 && v3.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v3.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v3.MaxY
			gapTex = v3.Texture
		}

		v4 := *(*Span)(unsafe.Add(p, 4*unsafe.Sizeof(Span{})))
		if (v4.Texture&ignoreTexture) == 0 && v4.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v4.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v4.MaxY
			gapTex = v4.Texture
		}

		v5 := *(*Span)(unsafe.Add(p, 5*unsafe.Sizeof(Span{})))
		if (v5.Texture&ignoreTexture) == 0 && v5.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v5.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v5.MaxY
			gapTex = v5.Texture
		}

		v6 := *(*Span)(unsafe.Add(p, 6*unsafe.Sizeof(Span{})))
		if (v6.Texture&ignoreTexture) == 0 && v6.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v6.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v6.MaxY
			gapTex = v6.Texture
		}

		v7 := *(*Span)(unsafe.Add(p, 7*unsafe.Sizeof(Span{})))
		if (v7.Texture&ignoreTexture) == 0 && v7.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v7.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v7.MaxY
			gapTex = v7.Texture
		}

		p = unsafe.Add(p, 8*unsafe.Sizeof(Span{}))
	}

	for uintptr(p) < uintptr(end) {
		v := *(*Span)(p)
		if (v.Texture&ignoreTexture) == 0 && v.MaxY > gapMinY {
			gMinI := int32(gapMinY)
			if gMinI > maxAllowed {
				goto FINAL
			}
			if gMinI >= minAllowed {
				gapMaxY := v.MinY
				if gapMaxY-gapMinY >= h16 && int32(gapMaxY) >= needTopY &&
					(forbiddenTexture == 0 || (gapTex&forbiddenTexture) == 0) {
					return Span{gapMinY, gapMaxY, gapTex}, true
				}
			}
			gapMinY = v.MaxY
			gapTex = v.Texture
		}
		p = unsafe.Add(p, unsafe.Sizeof(Span{}))
	}

FINAL:
	const finalMaxU16 = uint16(0xFFFF)
	finalMax := int32(finalMaxU16)

	if finalMax-int32(gapMinY) < int32(h16) {
		return
	}
	if finalMax < needTopY {
		return
	}
	if forbiddenTexture != 0 && (gapTex&forbiddenTexture) != 0 {
		return
	}
	gMinI := int32(gapMinY)
	if gMinI > maxAllowed || gMinI < minAllowed {
		return
	}
	return Span{gapMinY, finalMaxU16, gapTex}, true
}

// 构造测试数据：terrain.MaxY=10，spans=3000，每段高度20，头尾相接，第一个span.MinY=1，Texture=4
func buildTestData() (terrain Span, spans []Span) {
	terrain = Span{
		MinY:    0,
		MaxY:    10,
		Texture: 1, // terrain texture 可以随意，不影响你的构造要求
	}

	const n = 3000
	spans = make([]Span, n)

	minY := uint16(1)
	for i := 0; i < n; i++ {
		spans[i] = Span{
			MinY:    minY,
			MaxY:    minY + 20,
			Texture: 4,
		}
		minY += 20
	}

	return
}

func BenchmarkGetInterval_Normal(b *testing.B) {
	terrain, spans := buildTestData()

	curY := int32(100000)
	ignoreTexture := uint32(0)
	forbiddenTexture := uint32(0)
	height := int32(10)
	upLimit := int32(1000)
	downLimit := int32(1000)

	b.ReportAllocs()
	b.ResetTimer()

	var r Span
	var ok bool
	for i := 0; i < b.N; i++ {
		r, ok = GetInterval(terrain, spans, curY, ignoreTexture, forbiddenTexture, height, upLimit, downLimit)
	}

	// 防止编译器优化掉调用
	_ = r
	_ = ok
}

func BenchmarkGetInterval_ForbiddenHit(b *testing.B) {
	terrain, spans := buildTestData()

	terrain.Texture = 4

	curY := int32(100000)
	ignoreTexture := uint32(0)
	forbiddenTexture := uint32(4)
	height := int32(10)
	upLimit := int32(1000)
	downLimit := int32(1000)

	b.ReportAllocs()
	b.ResetTimer()

	var r Span
	var ok bool
	for i := 0; i < b.N; i++ {
		r, ok = GetInterval(terrain, spans, curY, ignoreTexture, forbiddenTexture, height, upLimit, downLimit)
	}

	_ = r
	_ = ok
}
