package main

import (
	"container/heap"
	"fmt"
	"math"
)

// ===================== Geometry / Grid =====================

type Pt struct{ X, Y int }

type Grid struct {
	W, H int
	Occ  [][]bool // Occ[y][x] == true means obstacle
}

func (g *Grid) InBounds(x, y int) bool { return x >= 0 && x < g.W && y >= 0 && y < g.H }
func (g *Grid) FreeCell(x, y int) bool { return g.InBounds(x, y) && !g.Occ[y][x] }

// 2x2 footprint: reference point is the bottom-left corner (x,y)
// Occupied cells: (x,y), (x+1,y), (x,y+1), (x+1,y+1)
func (g *Grid) FootprintFree(p Pt) bool {
	x, y := p.X, p.Y
	return g.FreeCell(x, y) &&
		g.FreeCell(x+1, y) &&
		g.FreeCell(x, y+1) &&
		g.FreeCell(x+1, y+1)
}

// Swept collision for a move p -> q:
// We "sample" along the motion in unit steps (r=1), checking 2x2 footprint each step.
// This prevents coarse layers from "jumping through" obstacles.
func (g *Grid) CollisionFree(p, q Pt) bool {
	dx := q.X - p.X
	dy := q.Y - p.Y
	steps := max(abs(dx), abs(dy))
	if steps == 0 {
		return g.FootprintFree(p)
	}
	// We assume moves are 8-connected with step r, so dx,dy are multiples of 1.
	// Interpolate in unit steps.
	stepX := sign(dx)
	stepY := sign(dy)

	cur := p
	// Check start as well (optional but safe)
	if !g.FootprintFree(cur) {
		return false
	}
	for i := 0; i < steps; i++ {
		cur = Pt{cur.X + stepX, cur.Y + stepY}
		if !g.FootprintFree(cur) {
			return false
		}
	}
	return true
}

// ===================== Priority Queue =====================

type Node struct {
	P      Pt
	G      float64
	F      float64
	H      float64
	Parent *Node
	index  int
}

type PQ []*Node

func (pq PQ) Len() int { return len(pq) }
func (pq PQ) Less(i, j int) bool {
	if pq[i].F == pq[j].F {
		return pq[i].G > pq[j].G
	}
	return pq[i].F < pq[j].F
}
func (pq PQ) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *PQ) Push(x any) {
	n := x.(*Node)
	n.index = len(*pq)
	*pq = append(*pq, n)
}
func (pq *PQ) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}
func (pq PQ) Peek() *Node {
	if len(pq) == 0 {
		return nil
	}
	return pq[0]
}

// ===================== MRA* Structures =====================

type Search struct {
	Step   int
	Weight float64 // w=1 for anchor, w=w1 for others
	Open   PQ

	G      map[int]float64 // key -> best g
	Closed map[int]bool    // key -> expanded?
	Best   map[int]*Node   // key -> best node for reconstruction
}

type MRAStar struct {
	Grid *Grid

	Steps []int
	W1    float64 // WA* weight
	W2    float64 // gating factor vs anchor

	AnchorIdx int
	Searches  []*Search

	Start Pt
	Goal  Pt
}

// key includes step to avoid collisions between spaces
func (m *MRAStar) key(p Pt, step int) int {
	// good enough for moderate map sizes
	return (step << 28) ^ (p.Y << 14) ^ p.X
}

func (m *MRAStar) heuristic(p Pt) float64 {
	dx := float64(p.X - m.Goal.X)
	dy := float64(p.Y - m.Goal.Y)
	return math.Hypot(dx, dy)
}

// coincide rule: p belongs to resolution step iff x%step==0 && y%step==0
func (m *MRAStar) getSpaceIndices(p Pt) []int {
	out := make([]int, 0, len(m.Steps))
	for i, st := range m.Steps {
		if p.X%st == 0 && p.Y%st == 0 {
			out = append(out, i)
		}
	}
	return out
}

func NewMRAStar2D(grid *Grid, start, goal Pt, steps []int, w1, w2 float64) *MRAStar {
	m := &MRAStar{
		Grid:  grid,
		Steps: steps,
		W1:    w1,
		W2:    w2,
		Start: start,
		Goal:  goal,
	}
	m.AnchorIdx = -1
	for i, st := range steps {
		if st == 1 {
			m.AnchorIdx = i
			break
		}
	}
	if m.AnchorIdx < 0 {
		panic("steps must include 1 as anchor resolution")
	}

	for i, st := range steps {
		s := &Search{
			Step:   st,
			Weight: w1,
			Open:   PQ{},
			G:      map[int]float64{},
			Closed: map[int]bool{},
			Best:   map[int]*Node{},
		}
		if i == m.AnchorIdx {
			s.Weight = 1.0 // anchor is admissible A*
		}
		heap.Init(&s.Open)
		m.Searches = append(m.Searches, s)
	}
	return m
}

// 8-connected neighbors with stride = step
func (m *MRAStar) neighbors(p Pt, step int) []Pt {
	dirs := []Pt{
		{step, 0}, {-step, 0}, {0, step}, {0, -step},
		{step, step}, {step, -step}, {-step, step}, {-step, -step},
	}
	out := make([]Pt, 0, 8)
	for _, d := range dirs {
		np := Pt{p.X + d.X, p.Y + d.Y}
		// quick bounds check for footprint (needs x+1,y+1)
		if m.Grid.InBounds(np.X, np.Y) && m.Grid.InBounds(np.X+1, np.Y+1) {
			out = append(out, np)
		}
	}
	return out
}

func moveCost(step int, diagonal bool) float64 {
	if diagonal {
		return float64(step) * math.Sqrt2
	}
	return float64(step)
}

// Insert/update in a specific search
func (m *MRAStar) pushOrUpdate(si int, p Pt, g float64, parent *Node) {
	s := m.Searches[si]
	k := m.key(p, s.Step)
	if old, ok := s.G[k]; ok && g >= old {
		return
	}
	s.G[k] = g
	h := m.heuristic(p)
	n := &Node{
		P:      p,
		G:      g,
		H:      h,
		F:      g + s.Weight*h,
		Parent: parent,
	}
	s.Best[k] = n
	heap.Push(&s.Open, n)
}

func (m *MRAStar) reconstruct(goalNode *Node) []Pt {
	path := []Pt{}
	for cur := goalNode; cur != nil; cur = cur.Parent {
		path = append(path, cur.P)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// ChooseQueue strategy: "eligible min-key"
// - If any non-anchor queue has minF <= w2*anchorMinF, pick the eligible one with smallest minF.
// - Otherwise pick anchor.
func (m *MRAStar) chooseQueue() int {
	anchor := m.Searches[m.AnchorIdx]
	if anchor.Open.Len() == 0 {
		return m.AnchorIdx
	}
	anchorPeek := anchor.Open.Peek()
	anchorMin := anchorPeek.G + anchorPeek.H
	anchorMin = anchorPeek.F

	bestIdx := m.AnchorIdx
	bestF := anchorMin

	for i, s := range m.Searches {
		if i == m.AnchorIdx || s.Open.Len() == 0 {
			continue
		}
		sPeek := s.Open.Peek()
		f := sPeek.G + sPeek.H
		f = sPeek.F
		if f <= m.W2*anchorMin {
			if bestIdx == m.AnchorIdx || f < bestF {
				bestIdx = i
				bestF = f
			}
		}
	}
	return bestIdx
}

func (m *MRAStar) Plan(maxExpansions int) ([]Pt, bool) {
	// Validate start/goal footprints
	if !m.Grid.FootprintFree(m.Start) || !m.Grid.FootprintFree(m.Goal) {
		return nil, false
	}

	// init: put start into every space it coincides with
	indices := m.getSpaceIndices(m.Start)
	for _, i := range indices {
		m.pushOrUpdate(i, m.Start, 0, nil)
	}

	expanded := make([]int, len(m.Searches))
	exp := 0
	for exp < maxExpansions {
		// stop if all opens empty
		allEmpty := true
		for _, s := range m.Searches {
			if s.Open.Len() > 0 {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			return nil, false
		}

		i := m.chooseQueue()
		sel := m.Searches[i]
		if sel.Open.Len() == 0 {
			// fallback: expand anchor if possible
			sel = m.Searches[m.AnchorIdx]
			i = m.AnchorIdx
			if sel.Open.Len() == 0 {
				return nil, false
			}
		}

		cur := heap.Pop(&sel.Open).(*Node)
		ck := m.key(cur.P, sel.Step)
		if sel.Closed[ck] {
			continue
		}
		sel.Closed[ck] = true
		exp++

		expanded[i]++
		// goal test: goal must coincide with this resolution to be reachable in this space
		if cur.P == m.Goal && i == m.AnchorIdx {
			// simplest/safest: return when anchor reaches exact goal reference
			fmt.Println("expanded per queue:", expanded)
			return m.reconstruct(cur), true
		}
		// If you want: allow non-anchor to return too (bounded-suboptimal), but then ensure your
		// termination condition matches your theoretical bound requirements.

		for _, nb := range m.neighbors(cur.P, sel.Step) {
			// Determine diagonal
			diag := (nb.X != cur.P.X) && (nb.Y != cur.P.Y)

			// Collision check with 2x2 footprint via swept sampling
			if !m.Grid.CollisionFree(cur.P, nb) {
				continue
			}

			nk := m.key(nb, sel.Step)
			if sel.Closed[nk] {
				continue
			}

			ng := cur.G + moveCost(sel.Step, diag)
			// update in this space
			m.pushOrUpdate(i, nb, ng, cur)

			// share to other spaces if coincide
			for _, j := range m.getSpaceIndices(nb) {
				if j == i {
					continue
				}
				// If already expanded in that space, don't reinsert
				if m.Searches[j].Closed[m.key(nb, m.Searches[j].Step)] {
					continue
				}
				// (Optional but safe) ensure footprint valid (it is, due to CollisionFree's final check)
				if !m.Grid.FootprintFree(nb) {
					continue
				}
				// share the same g-value as a "seed"
				m.pushOrUpdate(j, nb, ng, cur)
			}
		}
	}

	return nil, false
}

// ===================== Demo =====================

func main() {
	W, H := 30, 20
	occ := make([][]bool, H)
	for y := 0; y < H; y++ {
		occ[y] = make([]bool, W)
	}

	// Build a wall with a gap, but remember 2x2 footprint needs a 2-cell-wide passage effectively.
	// We'll create a corridor that a 2x2 robot can pass.
	for x := 5; x < 25; x++ {
		occ[10][x] = true
	}
	// Create a gap wide enough for 2x2 (clear two adjacent cells in wall row)
	occ[10][14] = false
	occ[10][15] = false

	grid := &Grid{W: W, H: H, Occ: occ}

	start := Pt{2, 2}
	goal := Pt{26, 16}

	steps := []int{1}
	w1 := 1.5
	w2 := 2.0

	mra := NewMRAStar2D(grid, start, goal, steps, w1, w2)
	path, ok := mra.Plan(500000)
	if !ok {
		fmt.Println("no path")
		return
	}
	fmt.Printf("found path, len=%d\n", len(path))
	for _, p := range path {
		fmt.Printf("(%d,%d) ", p.X, p.Y)
	}
	fmt.Println()
}

// ===================== Utils =====================

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func sign(a int) int {
	if a < 0 {
		return -1
	}
	if a > 0 {
		return 1
	}
	return 0
}
