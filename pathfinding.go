// ---------- pathfinding.go ----------
package main

import (
	"container/heap"
)

type Dir int8

const (
	E Dir = iota
	W
	N
	S
	NE
	NW
	SE
	SW
)

type AgentSpec struct {
	StepUp20    uint16 // = MaxStepUp20
	HeadClear20 uint16 // = HeadClear20
	IgnoreMask  TextureMask
}

type node struct {
	x, z    int32  // 宏格坐标
	h20     uint16 // 站立高度（1/20m）
	g, f    float32
	parent  *node
	openIdx int // heap 索引
}

type openHeap []*node

func (h openHeap) Len() int            { return len(h) }
func (h openHeap) Less(i, j int) bool  { return h[i].f < h[j].f }
func (h openHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i]; h[i].openIdx, h[j].openIdx = i, j }
func (h *openHeap) Push(x interface{}) { *h = append(*h, x.(*node)) }
func (h *openHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

type edgeCacheKey struct {
	x, z int32
	dir  Dir
	h20  uint16
	ign  TextureMask
}
type edgeCacheVal struct {
	ok bool
	nh uint16
}
type EdgeCache struct {
	// 简化实现：可换成 LRU（map+list），这里用 map 即可说明
	m map[edgeCacheKey]edgeCacheVal
}

func newEdgeCache() *EdgeCache { return &EdgeCache{m: make(map[edgeCacheKey]edgeCacheVal)} }

type Pathfinder struct {
	W  *World
	AG AgentSpec
	ec *EdgeCache
}

func NewPathfinder(w *World, ag AgentSpec) *Pathfinder {
	return &Pathfinder{W: w, AG: ag, ec: newEdgeCache()}
}

// FindPath 在宏格层面寻路（起终点用宏格坐标+起始高度）
func (pf *Pathfinder) FindPath(sx, sz int32, sh20 uint16, gx, gz int32) ([][3]float32, bool) {
	start := &node{x: sx, z: sz, h20: sh20, g: 0}
	start.f = pf.h(sx, sz, gx, gz)
	open := &openHeap{start}
	heap.Init(open)
	vis := make(map[int64]*node) // key = (x<<32)|uint32(z)
	vis[keyOf(sx, sz)] = start

	for open.Len() > 0 {
		cur := heap.Pop(open).(*node)
		if cur.x == gx && cur.z == gz {
			return pf.reconstruct(cur), true
		}
		for _, d := range []Dir{E, W, N, S, NE, NW, SE, SW} {
			nx, nz := step(cur.x, cur.z, d)
			nh, ok := pf.edgePass(cur.x, cur.z, cur.h20, d)
			if !ok {
				continue
			}
			// 反剪角（对角需要两正交边可行）
			if isDiag(d) {
				_, ok1 := pf.edgePass(cur.x, cur.z, cur.h20, toOrthA(d))
				_, ok2 := pf.edgePass(cur.x, cur.z, cur.h20, toOrthB(d))
				if !ok1 || !ok2 {
					continue
				}
			}
			ng := cur.g + moveCost(d)
			key := keyOf(nx, nz)
			if old, ok := vis[key]; ok {
				if ng < old.g {
					old.g = ng
					old.h20 = nh
					old.f = ng + pf.h(nx, nz, gx, gz)
					old.parent = cur
					if old.openIdx >= 0 {
						heap.Fix(open, old.openIdx)
					} else {
						heap.Push(open, old)
					}
				}
			} else {
				nn := &node{x: nx, z: nz, h20: nh, g: ng}
				nn.f = ng + pf.h(nx, nz, gx, gz)
				nn.parent = cur
				nn.openIdx = -1
				vis[key] = nn
				heap.Push(open, nn)
			}
		}
	}
	return nil, false
}

func (pf *Pathfinder) h(x, z, gx, gz int32) float32 {
	dx := float32(abs32(gx - x))
	dz := float32(abs32(gz - z))
	// Octile heuristic
	minv, maxv := dx, dz
	if minv > maxv {
		minv, maxv = maxv, minv
	}
	return (maxv - minv) + minv*sqrt2
}

const sqrt2 = 1.41421356

func moveCost(d Dir) float32 {
	if isDiag(d) {
		return sqrt2
	}
	return 1
}
func isDiag(d Dir) bool { return d >= NE }

func toOrthA(d Dir) Dir { // NE->E, NW->W, SE->E, SW->W
	switch d {
	case NE, SE:
		return E
	case NW, SW:
		return W
	}
	return d
}
func toOrthB(d Dir) Dir { // NE->N, NW->N, SE->S, SW->S
	switch d {
	case NE, NW:
		return N
	case SE, SW:
		return S
	}
	return d
}

func step(x, z int32, d Dir) (int32, int32) {
	switch d {
	case E:
		return x + 1, z
	case W:
		return x - 1, z
	case N:
		return x, z - 1
	case S:
		return x, z + 1
	case NE:
		return x + 1, z - 1
	case NW:
		return x - 1, z - 1
	case SE:
		return x + 1, z + 1
	case SW:
		return x - 1, z + 1
	}
	return x, z
}
func keyOf(x, z int32) int64 { return int64(x)<<32 | int64(uint32(z)) }
func abs32(a int32) int32 {
	if a < 0 {
		return -a
	}
	return a
}

// edgePass 判定从 (x,z,h20) 跨到方向 d 的相邻宏格中心是否可通行；
// 若可通行，返回目标站立高度 newH20（四子格 End 的最大值）。
func (pf *Pathfinder) edgePass(x, z int32, h20 uint16, d Dir) (uint16, bool) {
	key := edgeCacheKey{x: x, z: z, dir: d, h20: h20, ign: pf.AG.IgnoreMask}
	if v, ok := pf.ec.m[key]; ok {
		return v.nh, v.ok
	}

	// 源宏格中心四子格（quarter坐标）
	cxq := (x << 2) + 1
	czq := (z << 2) + 1
	// 目标宏格中心四子格
	tx := x
	tz := z
	switch d {
	case E:
		tx = x + 1
	case W:
		tx = x - 1
	case N:
		tz = z - 1
	case S:
		tz = z + 1
	case NE:
		tx, tz = x+1, z-1
	case NW:
		tx, tz = x-1, z-1
	case SE:
		tx, tz = x+1, z+1
	case SW:
		tx, tz = x-1, z+1
	}
	txq := (tx << 2) + 1
	tzq := (tz << 2) + 1

	// 目标宏格中心 4 个子格 quarter 坐标
	target := [4][2]int32{
		{txq + 0, tzq + 0},
		{txq + 0, tzq + 1},
		{txq + 1, tzq + 0},
		{txq + 1, tzq + 1},
	}
	var newH uint16
	for i := 0; i < 4; i++ {
		colID, ok := pf.W.columnAtQuarter(target[i][0], target[i][1])
		if !ok {
			pf.ec.m[key] = edgeCacheVal{ok: false}
			return 0, false
		}
		col := pf.W.cols.Get(colID)
		// 在该子格柱列上，寻找相对当前高度 h20 的最佳上表面 a
		top, ok := col.findBestSupport(h20, pf.AG.IgnoreMask)
		if !ok {
			pf.ec.m[key] = edgeCacheVal{ok: false}
			return 0, false
		}
		if top > newH {
			newH = top
		}
	}
	pf.ec.m[key] = edgeCacheVal{ok: true, nh: newH}
	return newH, true
}

// reconstruct 将宏路径还原为世界坐标（以 1×1 宏格中心为 xz，y=End/20）
func (pf *Pathfinder) reconstruct(goal *node) ([][3]float32, bool) {
	var rev [][3]float32
	for n := goal; n != nil; n = n.parent {
		x := float32(n.x) + 0.5
		z := float32(n.z) + 0.5
		y := float32(n.h20) / float32(HeightScale)
		rev = append(rev, [3]float32{x, y, z})
	}
	// reverse
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, true
}
