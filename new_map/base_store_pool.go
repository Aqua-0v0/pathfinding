package zmap3base

import (
	"golang.org/x/sync/syncmap"
	"math/bits"
	"sync/atomic"
	"time"
)

// Release 循环释放 BaseStore 内部资源
func (b *BaseStore) Release() {
	if b == nil {
		return
	}

	b.initRangeData = b.initRangeData[:0]
	_putGlobalRichRanges(cap(b.initRangeData), b.initRangeData)
	b.initRangeData = nil

	clear(b.rootCount)

	b.bucketData = b.bucketData[:0]
	_putGlobalBytes(cap(b.bucketData), b.bucketData)
	b.bucketData = nil
}

var _globalRichRangesMapPool syncmap.Map

// 实现一个基于channel的对象池
type RichRangeSlicePool struct {
	pool         chan []RichRange
	getCnt       atomic.Uint32
	putCnt       atomic.Uint32
	limitCnt     uint32
	cap          int
	recycleTimer *time.Timer
}

func NewRichRangeSlicePool(poolSize, sliceCap, limitCnt int) *RichRangeSlicePool {
	return &RichRangeSlicePool{
		pool:     make(chan []RichRange, poolSize),
		cap:      sliceCap,
		limitCnt: uint32(limitCnt),
	}
}

func (p *RichRangeSlicePool) Get() []RichRange {
	select {
	case data := <-p.pool:
		p.getCnt.Add(1)
		return data
	default:
		return make([]RichRange, 0, p.cap)
	}
}

func (p *RichRangeSlicePool) Put(data []RichRange) {
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

func (p *RichRangeSlicePool) Length() int {
	return len(p.pool)
}

func (p *RichRangeSlicePool) Capacity() int {
	return cap(p.pool)
}

// triggerRecycle 用于触发池的回收机制，每次回收10%
func (p *RichRangeSlicePool) triggerRecycle() {
	totalLen := len(p.pool)
	recycleCnt := totalLen / 10
	for i := 0; i < recycleCnt; i++ {
		p.Get()
	}

	// 重置计时器，30分钟后再次触发回收
	p.recycleTimer.Reset(time.Duration(30) * time.Minute)
}

func _getGlobalRichRanges(size int) []RichRange {
	if size == 0 {
		return make([]RichRange, 0)
	}
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if v, ok := _globalRichRangesMapPool.Load(key); ok {
		pl := v.(*RichRangeSlicePool)
		ret := pl.Get()
		return ret[:size]
	}

	pl := NewRichRangeSlicePool(10000, 1<<key, 5000)
	_globalRichRangesMapPool.Store(key, pl)
	ret := pl.Get()
	return ret[:size]
}

func _putGlobalRichRanges(size int, rrs []RichRange) {
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if size < 1<<key {
		key--
	}

	rrs = rrs[:1<<key]
	if v, ok := _globalRichRangesMapPool.Load(key); ok {
		pl := v.(*RichRangeSlicePool)
		pl.Put(rrs)
		return
	}

	pl := NewRichRangeSlicePool(10000, 1<<key, 5000)
	pl.Put(rrs)
	_globalRichRangesMapPool.Store(key, pl)
}

var _globalBytesMapPool syncmap.Map

// 实现一个基于channel的对象池
type BytesPool struct {
	pool         chan []byte
	getCnt       atomic.Uint32
	putCnt       atomic.Uint32
	limitCnt     uint32
	cap          int
	recycleTimer *time.Timer
}

func NewBytesPool(poolSize, sliceCap, limitCnt int) *BytesPool {
	return &BytesPool{
		pool:     make(chan []byte, poolSize),
		cap:      sliceCap,
		limitCnt: uint32(limitCnt),
	}
}

func (p *BytesPool) Get() []byte {
	select {
	case data := <-p.pool:
		p.getCnt.Add(1)
		return data
	default:
		return make([]byte, 0, p.cap)
	}
}

func (p *BytesPool) Put(data []byte) {
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

func (p *BytesPool) Length() int {
	return len(p.pool)
}

func (p *BytesPool) Capacity() int {
	return cap(p.pool)
}

// triggerRecycle 用于触发池的回收机制，每次回收10%
func (p *BytesPool) triggerRecycle() {
	totalLen := len(p.pool)
	recycleCnt := totalLen / 10
	for i := 0; i < recycleCnt; i++ {
		p.Get()
	}

	// 重置计时器，30分钟后再次触发回收
	p.recycleTimer.Reset(time.Duration(30) * time.Minute)
}

func _getGlobalBytes(size int) []byte {
	if size == 0 {
		return make([]byte, 0)
	}
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if v, ok := _globalBytesMapPool.Load(key); ok {
		pl := v.(*BytesPool)
		ret := pl.Get()
		return ret[:size]
	}

	pl := NewBytesPool(10000, 1<<key, 5000)
	_globalBytesMapPool.Store(key, pl)
	ret := pl.Get()
	return ret[:size]
}

func _putGlobalBytes(size int, rrs []byte) {
	key := bits.Len(uint(size))
	if size == 1<<(key-1) {
		key--
	}

	if size < 1<<key {
		key--
	}

	rrs = rrs[:1<<key]
	if v, ok := _globalBytesMapPool.Load(key); ok {
		pl := v.(*BytesPool)
		pl.Put(rrs)
		return
	}

	pl := NewBytesPool(10000, 1<<key, 5000)
	pl.Put(rrs)
	_globalBytesMapPool.Store(key, pl)
}
