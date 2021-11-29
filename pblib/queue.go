/**
 * 线程安全的队列，使用轻量级的 CAS 锁
 */
package m

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
)

type casCache struct {
	putNo uint32
	getNo uint32
	value interface{}
}

// lock free queue
type CASQueue struct {
	capacity uint32
	capMod   uint32
	putPos   uint32
	getPos   uint32
	cache    []casCache
}

func NewCASQueue(capacity uint32) *CASQueue {
	q := new(CASQueue)
	q.capacity = minQuantity(capacity)
	q.capMod = q.capacity - 1
	q.putPos = 0
	q.getPos = 0
	q.cache = make([]casCache, q.capacity)
	for i := range q.cache {
		cache := &q.cache[i]
		cache.getNo = uint32(i)
		cache.putNo = uint32(i)
	}
	cache := &q.cache[0]
	cache.getNo = q.capacity
	cache.putNo = q.capacity
	return q
}

func (q *CASQueue) String() string {
	getPos := atomic.LoadUint32(&q.getPos)
	putPos := atomic.LoadUint32(&q.putPos)
	return fmt.Sprintf("Queue{capacity: %v, capMod: %v, putPos: %v, getPos: %v}",
		q.capacity, q.capMod, putPos, getPos)
}

func (q *CASQueue) getCapacity() uint32 {
	return q.capacity
}

/**
* 获取当前元素个数
 */
func (q *CASQueue) Quantity() uint32 {
	var putPos, getPos uint32
	var quantity uint32
	getPos = atomic.LoadUint32(&q.getPos)
	putPos = atomic.LoadUint32(&q.putPos)

	if putPos >= getPos {
		quantity = putPos - getPos
	} else {
		quantity = q.capMod + (putPos - getPos)
	}

	return quantity
}

/**
* put queue functions
* ok: 如果成功添加，则为true，反之同一时间有多个线程put导致写入失败或者队列长度不够，则返回false
* quantity: 返回代表队列的元素个数，如果大于等于capMod - 1则表示空间满了
 */
func (q *CASQueue) putMayFail(val interface{}) (ok bool, quantity uint32) {
	var putPos, putPosNew, getPos, posCnt uint32
	var cache *casCache
	capMod := q.capMod

	getPos = atomic.LoadUint32(&q.getPos)
	putPos = atomic.LoadUint32(&q.putPos)

	if putPos >= getPos {
		posCnt = putPos - getPos
	} else {
		posCnt = capMod + (putPos - getPos)
	}

	// 空间不足
	if posCnt >= capMod-1 {
		runtime.Gosched()
		return false, posCnt
	}

	putPosNew = putPos + 1
	if !atomic.CompareAndSwapUint32(&q.putPos, putPos, putPosNew) {
		runtime.Gosched()
		return false, posCnt
	}

	cache = &q.cache[putPosNew&capMod]

	for {
		getNo := atomic.LoadUint32(&cache.getNo)
		putNo := atomic.LoadUint32(&cache.putNo)
		if putPosNew == putNo && getNo == putNo {
			cache.value = val
			atomic.AddUint32(&cache.putNo, q.capacity)
			return true, posCnt + 1
		} else {
			runtime.Gosched()
		}
	}
}

/**
* 添加一个元素到队列，如果队列满了则报错
 */
func (q *CASQueue) Put(val interface{}) error {
	var ok bool
	var quantity uint32
	for !ok { // 写入失败，没拿到CAS锁，则继续写入
		ok, quantity = q.putMayFail(val)
		// 队列长度不够了，则直接返回错误
		if quantity >= q.capMod-1 {
			errMsg := fmt.Sprintf("queue almost overflow, the capacity is [%d], now the quantity is [%d]", q.capacity, quantity)
			return errors.New(errMsg)
		}
	}
	return nil
}

/**
* 获取队列中的数据
* ok: 获取成功为 true，否则false
* quantity: 当前剩下的数据量，为0且ok为false则说明没有数据可读了
 */
func (q *CASQueue) getMayFail() (val interface{}, ok bool, quantity uint32) {
	var putPos, getPos, getPosNew, posCnt uint32
	var cache *casCache
	capMod := q.capMod

	putPos = atomic.LoadUint32(&q.putPos)
	getPos = atomic.LoadUint32(&q.getPos)

	if putPos >= getPos {
		posCnt = putPos - getPos
	} else {
		posCnt = capMod + (putPos - getPos)
	}

	if posCnt < 1 {
		runtime.Gosched()
		return nil, false, posCnt
	}

	getPosNew = getPos + 1
	if !atomic.CompareAndSwapUint32(&q.getPos, getPos, getPosNew) {
		runtime.Gosched()
		return nil, false, posCnt
	}

	cache = &q.cache[getPosNew&capMod]

	for {
		getNo := atomic.LoadUint32(&cache.getNo)
		putNo := atomic.LoadUint32(&cache.putNo)
		if getPosNew == getNo && getNo == putNo-q.capacity {
			val = cache.value
			cache.value = nil
			atomic.AddUint32(&cache.getNo, q.capacity)
			return val, true, posCnt - 1
		} else {
			runtime.Gosched()
		}
	}
}

/**
* 获取队列中的数据
* emptyFlag: false 则此次没获取到数据，原因是队列为空，true则获取到了数据
 */
func (q *CASQueue) Get() (val interface{}, emptyFlag bool) {
	var ok bool
	var quantity uint32
	var v interface{}
	for !ok { // 写入失败，没拿到CAS锁，则继续写入
		v, ok, quantity = q.getMayFail()
		// 队列为空
		if quantity == 0 && !ok {
			return nil, false
		}
	}
	return v, true
}

// round 到最近的2的倍数
func minQuantity(v uint32) uint32 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}
