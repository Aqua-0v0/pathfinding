package zmap3base

import (
	"golang.org/x/sync/syncmap"
	"math/bits"
	"sync/atomic"
	"time"
)

var __highPrecisionColumnPool = NewHighPrecisionColumnPool(10000, 5000)

func GetHighPrecisionColumnFromPool() *HighPrecisionColumn {
	return __highPrecisionColumnPool.Get()
}

func PutHighPrecisionColumnToPool(data *HighPrecisionColumn) {
	__highPrecisionColumnPool.Put(data)
}

// Release 循环释放 RichRangeSetData 内部资源
func (r *RichRangeSetData) Release() {
	if r == nil {
		return
	}

	r.RootNode = 0
	r.Climate = 0

	if r.HighPrecision != nil {
		r.HighPrecision.Release()
		r.HighPrecision = nil
	}
}

// Release HighPrecisionColumn 内部资源
func (h *HighPrecisionColumn) Release() {
	if h == nil {
		return
	}

	h.Has = 0
	h.Same = 0
	h.Spans = h.Spans[:0]
	_putGlobalSpans(cap(h.Spans), h.Spans)
	h.Spans = nil
	PutHighPrecisionColumnToPool(h)
}

// 实现一个基于channel的对象池
type HighPrecisionColumnPool struct {
	pool         chan *HighPrecisionColumn
	getCnt       atomic.Uint32
	putCnt       atomic.Uint32
	limitCnt     uint32
	recycleTimer *time.Timer
}

func NewHighPrecisionColumnPool(poolSize, limitCnt int) *HighPrecisionColumnPool {
	return &HighPrecisionColumnPool{
		pool:     make(chan *HighPrecisionColumn, poolSize),
		limitCnt: uint32(limitCnt),
	}
}

func (p *HighPrecisionColumnPool) Get() *HighPrecisionColumn {
	select {
	case data := <-p.pool:
		p.getCnt.Add(1)
		return data
	default:
		return new(HighPrecisionColumn)
	}
}

func (p *HighPrecisionColumnPool) Put(data *HighPrecisionColumn) {
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

func (p *HighPrecisionColumnPool) Length() int {
	return len(p.pool)
}

func (p *HighPrecisionColumnPool) Capacity() int {
	return cap(p.pool)
}

// triggerRecycle 用于触发池的回收机制，每次回收10%
func (p *HighPrecisionColumnPool) triggerRecycle() {
	totalLen := len(p.pool)
	recycleCnt := totalLen / 10
	for i := 0; i < recycleCnt; i++ {
		p.Get()
	}

	// 重置计时器，30分钟后再次触发回收
	p.recycleTimer.Reset(time.Duration(30) * time.Minute)
}

var _globalSpansMapPool syncmap.Map

// 实现一个基于channel的对象池
type SpansPool struct {
	pool         chan []int32
	getCnt       atomic.Uint32
	putCnt       atomic.Uint32
	limitCnt     uint32
	cap          int
	recycleTimer *time.Timer
}

func NewSpansPool(poolSize, sliceCap, limitCnt int) *SpansPool {
	return &SpansPool{
		pool:     make(chan []int32, poolSize),
		cap:      sliceCap,
		limitCnt: uint32(limitCnt),
	}
}

func (p *SpansPool) Get() []int32 {
	select {
	case data := <-p.pool:
		p.getCnt.Add(1)
		return data
	default:
		return make([]int32, 0, p.cap)
	}
}

func (p *SpansPool) Put(data []int32) {
	select {
	case p.pool <- data:
		p.putCnt.Add(1)
	default:
		// 池已满，丢弃
	}

	getCnt := p.getCnt.Load()
	putCnt := p.putCnt.Load()
	//log.Errorf("SpansPool Put getCnt=%d, putCnt=%d, len=%d, limitCnt=%d", getCnt, putCnt, len(p.pool), p.limitCnt)
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

func (p *SpansPool) Length() int {
	return len(p.pool)
}

func (p *SpansPool) Capacity() int {
	return cap(p.pool)
}

// triggerRecycle 用于触发池的回收机制，每次回收10%
func (p *SpansPool) triggerRecycle() {
	totalLen := len(p.pool)
	recycleCnt := totalLen / 10
	for i := 0; i < recycleCnt; i++ {
		p.Get()
	}

	// 重置计时器，30分钟后再次触发回收
	p.recycleTimer.Reset(time.Duration(30) * time.Minute)
}

func _getGlobalSpans(size int) []int32 {
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if v, ok := _globalSpansMapPool.Load(key); ok {
		pl := v.(*SpansPool)
		ret := pl.Get()
		return ret[:size]
	}

	pl := NewSpansPool(10000, 1<<key, 5000)
	_globalSpansMapPool.Store(key, pl)
	ret := pl.Get()
	return ret[:size]
}

func _putGlobalSpans(size int, rrs []int32) {
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if size < 1<<key {
		key--
	}

	rrs = rrs[:1<<key]
	if v, ok := _globalSpansMapPool.Load(key); ok {
		pl := v.(*SpansPool)
		pl.Put(rrs)
		return
	}

	pl := NewSpansPool(10000, 1<<key, 5000)
	pl.Put(rrs)
	_globalSpansMapPool.Store(key, pl)
}
