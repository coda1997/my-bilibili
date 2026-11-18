package main

import (
	"sync"
	"time"
)

// One acknowledged dispatch advances domain ownership even without a paint.
// Stop never waits on the UI thread; Done belongs to the final close observer.
type clientPump struct {
	stop, done chan struct{}
	once       sync.Once
}

func startClientPump(dispatch func(func()), tick func(), every time.Duration) *clientPump {
	p := &clientPump{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		timer := time.NewTicker(every)
		defer timer.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-timer.C:
			}
			ack := make(chan struct{})
			dispatch(func() {
				defer close(ack)
				select {
				case <-p.stop:
					return
				default:
					tick()
				}
			})
			select {
			case <-p.stop:
				return
			case <-ack:
			}
		}
	}()
	return p
}
func (p *clientPump) Stop()                 { p.once.Do(func() { close(p.stop) }) }
func (p *clientPump) Done() <-chan struct{} { return p.done }
