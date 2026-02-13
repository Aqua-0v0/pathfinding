package zmap3base

import (
	"sync/atomic"
	"time"
)

var __gridRBDataPool = NewGridRBDataPool(10000, 5000)

func GetGridRBDataFromPool() *GridRBData {
	return __gridRBDataPool.Get()
}

func PutGridRBDataToPool(data *GridRBData) {
	__gridRBDataPool.Put(data)
}

// Release 循环释放 GridRBData 内部资源
func (g *GridRBData) Release() {
	if g == nil {
		return
	}

	g.baseX = 0
	g.baseY = 0

	for i := 0; i < len(g.cells); i++ {
		g.cells[i].Release()
	}

	g.base.Release()

	g.dirtyPool.Release()
	g.dirtyPool = nil

	g.dirtyOps.pool = nil

	PutGridRBDataToPool(g)
}

// 实现一个基于channel的对象池
type GridRBDataPool struct {
	pool         chan *GridRBData
	getCnt       atomic.Uint32
	putCnt       atomic.Uint32
	limitCnt     uint32
	recycleTimer *time.Timer
}

func NewGridRBDataPool(poolSize, limitCnt int) *GridRBDataPool {
	return &GridRBDataPool{
		pool:     make(chan *GridRBData, poolSize),
		limitCnt: uint32(limitCnt),
	}
}

func (p *GridRBDataPool) Get() *GridRBData {
	select {
	case data := <-p.pool:
		p.getCnt.Add(1)
		return data
	default:
		return new(GridRBData)
	}
}

func (p *GridRBDataPool) Put(data *GridRBData) {
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

func (p *GridRBDataPool) Length() int {
	return len(p.pool)
}

func (p *GridRBDataPool) Capacity() int {
	return cap(p.pool)
}

// triggerRecycle 用于触发池的回收机制，每次回收10%
func (p *GridRBDataPool) triggerRecycle() {
	totalLen := len(p.pool)
	recycleCnt := totalLen / 10
	for i := 0; i < recycleCnt; i++ {
		p.Get()
	}
	// 重置计时器，30分钟后再次触发回收
	p.recycleTimer.Reset(time.Duration(30) * time.Minute)
}
