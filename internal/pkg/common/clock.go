package common

import (
	"sync/atomic"
	"time"
)

type Clock interface {
	Now() int64
}

// StoppableClock a clock impl that must be stopped to cleanup its resources
type StoppableClock interface {
	Clock

	Stop()
}

// CachedClock stores Unix time every second and returns the cached value
type CachedClock struct {
	epoch  int64
	ticker *time.Ticker
	done   chan bool
}

func NewCachedClock() StoppableClock {
	c := &CachedClock{
		epoch:  epoch(),
		ticker: time.NewTicker(time.Second),
		done:   make(chan bool),
	}

	go c.tick()

	return c
}

func (c *CachedClock) Now() int64 {
	return atomic.LoadInt64(&c.epoch)
}

func (c *CachedClock) Stop() {
	c.ticker.Stop()
	c.done <- true
	close(c.done)

	c.done = nil
	c.ticker = nil
}

// Periodically check and update cached time
func (c *CachedClock) tick() {
	for {
		select {
		case <-c.done:
			return
		case <-c.ticker.C:
			atomic.StoreInt64(&c.epoch, epoch())
		}
	}
}

func epoch() int64 {
	return time.Now().Unix()
}
