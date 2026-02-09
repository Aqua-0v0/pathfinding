// ---------- world_store.go ----------
package main

import (
	"encoding/binary"
	"hash/fnv"
	"sync"
)

type ColumnID uint32

// World 仅存活已加载的 Chunk；柱列全局驻留去重
type World struct {
	mu       sync.RWMutex
	chunks   map[ChunkKey]*Chunk
	cols     columnStore
	chunkDim int // 固定 32
}

func NewWorld() *World {
	return &World{
		chunks:   make(map[ChunkKey]*Chunk),
		cols:     newColumnStore(),
		chunkDim: 32,
	}
}

// ---- 柱列驻留 ----

type columnStore struct {
	mu     sync.RWMutex
	nextID ColumnID
	data   map[ColumnID]*RichRangeSetData
	byHash map[uint64][]ColumnID
}

func newColumnStore() columnStore {
	return columnStore{
		data:   make(map[ColumnID]*RichRangeSetData),
		byHash: make(map[uint64][]ColumnID),
	}
}

func hashRichRange(rr []RichRange) uint64 {
	h := fnv.New64a()
	var buf [12]byte
	for _, r := range rr {
		binary.LittleEndian.PutUint16(buf[0:], r.Begin)
		binary.LittleEndian.PutUint16(buf[2:], r.End)
		binary.LittleEndian.PutUint32(buf[4:], uint32(r.Texture))
		binary.LittleEndian.PutUint32(buf[8:], r.Config)
		_, _ = h.Write(buf[:])
	}
	return h.Sum64()
}

func (cs *columnStore) Intern(col *RichRangeSetData) ColumnID {
	col.Normalize()
	h := hashRichRange(col.raw)
	cs.mu.RLock()
	ids := cs.byHash[h]
	for _, id := range ids {
		if equalColumns(cs.data[id], col) {
			cs.mu.RUnlock()
			return id
		}
	}
	cs.mu.RUnlock()

	cs.mu.Lock()
	defer cs.mu.Unlock()
	// double-check
	for _, id := range cs.byHash[h] {
		if equalColumns(cs.data[id], col) {
			return id
		}
	}
	id := cs.nextID
	cs.nextID++
	clone := &RichRangeSetData{raw: append([]RichRange(nil), col.raw...)}
	cs.data[id] = clone
	cs.byHash[h] = append(cs.byHash[h], id)
	return id
}

func (cs *columnStore) Get(id ColumnID) *RichRangeSetData {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.data[id]
}

func equalColumns(a, b *RichRangeSetData) bool {
	if len(a.raw) != len(b.raw) {
		return false
	}
	for i := range a.raw {
		if a.raw[i] != b.raw[i] {
			return false
		}
	}
	return true
}

// ---- Chunk/Tile 定义 ----

type ChunkKey struct{ X, Z int32 }

type tileKind uint8

const (
	tileUniform tileKind = 1
	tileSubdiv  tileKind = 2
)

// SubdivTile 细分宏格的 4×4 子格（易读实现；工程化可 4bit 压缩成 8B）
type SubdivTile struct {
	Palette []ColumnID // ≤16
	Index   [16]uint8  // 16 子格的调色板索引
}

func (s *SubdivTile) At(ix, iz int) ColumnID {
	return s.Palette[s.Index[iz*4+ix]]
}

type Tile struct {
	Kind    tileKind
	Uniform ColumnID    // Kind=tileUniform
	Subdiv  *SubdivTile // Kind=tileSubdiv
}

type Chunk struct {
	Dim   int      // 32
	Cells [][]Tile // [Dim][Dim]
}

func NewChunk(dim int) *Chunk {
	c := &Chunk{Dim: dim}
	c.Cells = make([][]Tile, dim)
	for i := range c.Cells {
		c.Cells[i] = make([]Tile, dim)
	}
	return c
}

// ---- World 坐标查询 ----

// quarter 坐标：以 0.25 为步长的整数网格坐标。
func (w *World) columnAtQuarter(xq, zq int32) (ColumnID, bool) {
	tileX, subX := xq>>2, int(xq&3)
	tileZ, subZ := zq>>2, int(zq&3)

	chX, inX := tileX>>5, int(tileX&31) // 32=2^5
	chZ, inZ := tileZ>>5, int(tileZ&31)
	key := ChunkKey{X: chX, Z: chZ}

	w.mu.RLock()
	ch := w.chunks[key]
	w.mu.RUnlock()
	if ch == nil {
		return 0, false
	}
	t := &ch.Cells[inX][inZ]
	switch t.Kind {
	case tileUniform:
		return t.Uniform, true
	case tileSubdiv:
		return t.Subdiv.At(subX, subZ), true
	default:
		return 0, false
	}
}

func (w *World) SetUniform(tileX, tileZ int32, col *RichRangeSetData) {
	id := w.cols.Intern(col)
	w.ensureChunk(tileX>>5, tileZ>>5)
	ch := w.chunks[ChunkKey{X: tileX >> 5, Z: tileZ >> 5}]
	ch.Cells[tileX&31][tileZ&31] = Tile{Kind: tileUniform, Uniform: id}
}

func (w *World) SetSubdiv(tileX, tileZ int32, pal []ColumnID, idx [16]uint8) {
	w.ensureChunk(tileX>>5, tileZ>>5)
	ch := w.chunks[ChunkKey{X: tileX >> 5, Z: tileZ >> 5}]
	ch.Cells[tileX&31][tileZ&31] = Tile{
		Kind:   tileSubdiv,
		Subdiv: &SubdivTile{Palette: append([]ColumnID(nil), pal...), Index: idx},
	}
}

func (w *World) ensureChunk(chX, chZ int32) {
	key := ChunkKey{X: chX, Z: chZ}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.chunks[key] == nil {
		w.chunks[key] = NewChunk(32)
	}
}
