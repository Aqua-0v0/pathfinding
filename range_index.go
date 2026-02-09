// ---------- range_index.go ----------
package main

import (
	"sort"
)

const (
	// 体素高度单位换算：真实高度 = value / 20.0
	HeightScale = 20
	MaxStepUp20 = 1 * HeightScale // 允许上台阶 ≤ 1.0m
	HeadClear20 = 36              // 1.8m 抬头空间 = 1.8*20 = 36
)

type TextureMask uint64

func (m TextureMask) Has(t Texture) bool { return (m & (1 << (uint64(t) & 63))) != 0 }

// Normalize 对柱列数据进行排序+归并，生成严格有序、无重叠的阻挡段。
func (s *RichRangeSetData) Normalize() {
	if len(s.raw) <= 1 {
		return
	}
	sort.SliceStable(s.raw, func(i, j int) bool {
		if s.raw[i].End == s.raw[j].End {
			return s.raw[i].Begin < s.raw[j].Begin
		}
		return s.raw[i].End < s.raw[j].End
	})
	merged := s.raw[:0]
	for _, rr := range s.raw {
		n := len(merged)
		if n == 0 {
			merged = append(merged, rr)
			continue
		}
		last := &merged[n-1]
		// 可归并：同材质、相交或首尾相接
		if last.Texture == rr.Texture && rr.Begin <= last.End {
			if rr.End > last.End {
				last.End = rr.End
			}
			if rr.Begin < last.Begin {
				last.Begin = rr.Begin
			}
			continue
		}
		merged = append(merged, rr)
	}
	s.raw = merged
}

// findBestSupport 在给定当前高度 h20（单位=1/20m）的前提下，寻找目标“上表面 a”。
// 先尝试上台阶（≤1m），否则尝试下台阶。抬头空间判定可忽略部分材质（仅用于抬头，承重不忽略）。
func (s *RichRangeSetData) findBestSupport(h20 uint16, ignore TextureMask) (topEnd uint16, ok bool) {
	if len(s.raw) == 0 {
		return 0, false
	}
	// 先保证有序
	s.Normalize()

	// 1) 上台阶：End ∈ [h20, h20 + MaxStepUp20]
	upper := uint32(h20) + uint32(MaxStepUp20)
	idx := sort.Search(len(s.raw), func(i int) bool {
		return uint32(s.raw[i].End) > upper
	})
	// idx 是第一个 End > upper 的位置，所以 [0, idx) 都满足 End ≤ upper
	for i := idx - 1; i >= 0; i-- {
		e := s.raw[i].End
		if e < h20 {
			break
		}
		if s.hasHeadroomAbove(i, e, ignore) {
			return e, true
		}
	}

	// 2) 下台阶：End < h20，取最高且有抬头空间
	idx2 := sort.Search(len(s.raw), func(i int) bool {
		return s.raw[i].End >= h20
	})
	for i := idx2 - 1; i >= 0; i-- {
		e := s.raw[i].End
		if s.hasHeadroomAbove(i, e, ignore) {
			return e, true
		}
	}
	return 0, false
}

// hasHeadroomAbove 从第 i 段的上表面 end 向上查找下一个“未被忽略材质”的阻挡段起点 Begin，
// 要求 Begin - end >= 1.8m(=36) 才算有抬头空间；若上方再无未忽略阻挡，则视为无限抬头，判定通过。
func (s *RichRangeSetData) hasHeadroomAbove(i int, end uint16, ignore TextureMask) bool {
	for j := i + 1; j < len(s.raw); j++ {
		tex := s.raw[j].Texture
		if ignore.Has(tex) {
			continue // 忽略该材质阻挡
		}
		beg := s.raw[j].Begin
		delta := int32(beg) - int32(end)
		return delta >= HeadClear20
	}
	// 上方没有未忽略阻挡
	return true
}
