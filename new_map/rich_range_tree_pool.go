package zmap3base

import (
	"golang.org/x/sync/syncmap"
	"log"
	"math/bits"
	"sync/atomic"
	"time"
)

var __nodePoolPool = NewNodePoolPool(10000, 5000)

var _globalRichRangeNodeSliceMapPool syncmap.Map

// 实现一个基于channel的对象池
type RichRangeNodeSlicePool struct {
	pool         chan []RichRangeNode
	getCnt       atomic.Uint32
	putCnt       atomic.Uint32
	limitCnt     uint32
	cap          int
	recycleTimer *time.Timer
}

// 实现一个基于channel的对象池
type NodePoolPool struct {
	pool         chan *NodePool
	getCnt       atomic.Uint32
	putCnt       atomic.Uint32
	limitCnt     uint32
	recycleTimer *time.Timer
}

func GetNodePoolFromPool() *NodePool {
	return __nodePoolPool.Get()
}

func PutNodePoolToPool(data *NodePool) {
	__nodePoolPool.Put(data)
}

func NewNodePoolPool(poolSize, limitCnt int) *NodePoolPool {
	return &NodePoolPool{
		pool:     make(chan *NodePool, poolSize),
		limitCnt: uint32(limitCnt),
	}
}

func NewRichRangeNodeSlicePool(poolSize, sliceCap, limitCnt int) *RichRangeNodeSlicePool {
	return &RichRangeNodeSlicePool{
		pool:     make(chan []RichRangeNode, poolSize),
		cap:      sliceCap,
		limitCnt: uint32(limitCnt),
	}
}

func (p *NodePoolPool) Get() *NodePool {
	select {
	case data := <-p.pool:
		p.getCnt.Add(1)
		return data
	default:
		return new(NodePool)
	}
}

func (p *NodePoolPool) Put(data *NodePool) {
	select {
	case p.pool <- data:
		p.putCnt.Add(1)
	default:
		// 池已满，丢弃
	}

	getCnt := p.getCnt.Load()
	putCnt := p.putCnt.Load()
	//log.Errorf("RichRangeSlicePool Put getCnt=%d, putCnt=%d, len=%d, limitCnt=%d", getCnt, putCnt, len(p.pool), p.limitCnt)
	if putCnt-getCnt > p.limitCnt {
		// 触发回收机制
		if p.recycleTimer == nil {
			p.recycleTimer = time.AfterFunc(time.Duration(60)*time.Second, func() {
				p.triggerRecycle()
			})
		} else {
			p.recycleTimer.Reset(time.Duration(60) * time.Second)
		}
	} else {
		if p.recycleTimer != nil {
			p.recycleTimer.Stop()
		}
	}
}

func (p *NodePoolPool) Length() int {
	return len(p.pool)
}

func (p *NodePoolPool) Capacity() int {
	return cap(p.pool)
}

// triggerRecycle 用于触发池的回收机制，每次回收10%
func (p *NodePoolPool) triggerRecycle() {
	log.Printf("NodePoolPool triggerRecycle start len=%d", len(p.pool))
	totalLen := len(p.pool)
	recycleCnt := totalLen / 10
	for i := 0; i < recycleCnt; i++ {
		p.Get()
	}
	log.Printf("NodePoolPool triggerRecycle end len=%d", len(p.pool))

	// 重置计时器，30分钟后再次触发回收
	p.recycleTimer.Reset(time.Duration(30) * time.Minute)
}

func (p *RichRangeNodeSlicePool) Get() []RichRangeNode {
	select {
	case data := <-p.pool:
		p.getCnt.Add(1)
		return data
	default:
		return make([]RichRangeNode, 0, p.cap)
	}
}

func (p *RichRangeNodeSlicePool) Put(data []RichRangeNode) {
	select {
	case p.pool <- data:
		p.putCnt.Add(1)
	default:
		// 池已满，丢弃
	}

	getCnt := p.getCnt.Load()
	putCnt := p.putCnt.Load()
	//log.Errorf("RichRangeSlicePool Put getCnt=%d, putCnt=%d, len=%d, limitCnt=%d", getCnt, putCnt, len(p.pool), p.limitCnt)
	if putCnt-getCnt > p.limitCnt {
		// 触发回收机制
		if p.recycleTimer == nil {
			p.recycleTimer = time.AfterFunc(time.Duration(60)*time.Second, func() {
				p.triggerRecycle()
			})
		} else {
			p.recycleTimer.Reset(time.Duration(60) * time.Second)
		}
	} else {
		if p.recycleTimer != nil {
			p.recycleTimer.Stop()
		}
	}
}

func (p *RichRangeNodeSlicePool) Length() int {
	return len(p.pool)
}

func (p *RichRangeNodeSlicePool) Capacity() int {
	return cap(p.pool)
}

// triggerRecycle 用于触发池的回收机制，每次回收10%
func (p *RichRangeNodeSlicePool) triggerRecycle() {
	log.Printf("RichRangeNodeSlicePool triggerRecycle start len=%d", len(p.pool))
	totalLen := len(p.pool)
	recycleCnt := totalLen / 10
	for i := 0; i < recycleCnt; i++ {
		p.Get()
	}
	log.Printf("RichRangeNodeSlicePool triggerRecycle end len=%d", len(p.pool))

	// 重置计时器，30分钟后再次触发回收
	p.recycleTimer.Reset(time.Duration(30) * time.Minute)
}

// Release 释放NodePool内部资源
func (p *NodePool) Release() {
	if p == nil {
		return
	}
	if len(p.nodes) == 0 {
		p.freeHead = 0
		return
	}
	p.freeHead = 0
	p.nodes = p.nodes[:0]
	_putGlobalRichRangeNodeSlice(cap(p.nodes), p.nodes)
	p.nodes = nil
	PutNodePoolToPool(p)
}

func _getGlobalRichRangeNodeSlice(size int) []RichRangeNode {
	if size == 0 {
		return make([]RichRangeNode, 0)
	}
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if v, ok := _globalRichRangeNodeSliceMapPool.Load(key); ok {
		pl := v.(*RichRangeNodeSlicePool)
		ret := pl.Get()
		return ret[:size]
	}

	pl := NewRichRangeNodeSlicePool(10000, 1<<key, 5000)
	_globalRichRangeNodeSliceMapPool.Store(key, pl)
	ret := pl.Get()
	return ret[:size]
}

func _putGlobalRichRangeNodeSlice(size int, rrs []RichRangeNode) {
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if size < 1<<key {
		key--
	}

	rrs = rrs[:1<<key]
	if v, ok := _globalRichRangeNodeSliceMapPool.Load(key); ok {
		pl := v.(*RichRangeNodeSlicePool)
		pl.Put(rrs)
		return
	}

	pl := NewRichRangeNodeSlicePool(10000, 1<<key, 5000)
	pl.Put(rrs)
	_globalRichRangeNodeSliceMapPool.Store(key, pl)
}
