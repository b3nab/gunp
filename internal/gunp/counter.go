package gunp

import "sync/atomic"

type Counter struct {
	Count     atomic.Int64
	UpdatedCh chan struct{}
}

func NewCounter() *Counter {
	return NewCounterWithChannel(make(chan struct{}, 1))
}

func NewCounterWithChannel(updatedCh chan struct{}) *Counter {
	return &Counter{
		UpdatedCh: updatedCh,
	}
}

func (c *Counter) Add(n int64) {
	c.Count.Add(n)
	select {
	case c.UpdatedCh <- struct{}{}:
		// Notification sent successfully via channel.
	default:
		// Channel is full. Skip the notification update.
	}
}

func (c *Counter) Get() int64 {
	return c.Count.Load()
}

func (c *Counter) Close() {
	close(c.UpdatedCh)
}
