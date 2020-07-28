package gosd

import (
	"time"
)

type delayer interface {
	stop(drain bool)
	wait(msg *ScheduledMessage)
	available() bool
}

type delayState int

const (
	Idle delayState = iota
	Waiting
)

type delay struct {
	state            delayState
	respondIdleState bool

	idleChannel   chan<- bool
	egressChannel chan<- interface{}
	cancelChannel chan bool
}

func newDelay(respondIdleState bool, egressChannel chan<- interface{}, idleChannel chan<- bool) *delay {
	return &delay{
		respondIdleState: respondIdleState,
		idleChannel:      idleChannel,
		egressChannel:    egressChannel,
		cancelChannel:    make(chan bool, 1),
	}
}

// stop sends a cancel signal to the current timer process
func (d *delay) stop(drain bool) {
	if d.state == Waiting {
		d.cancelChannel <- drain
	}
}

// wait will create a timer based on the time from `msg.At` and dispatch the message to the egress channel asynchronously
func (d *delay) wait(msg *ScheduledMessage) {
	d.state = Waiting
	curTimer := time.NewTimer(msg.At.Sub(time.Now()))

	go func() {
		for {
			select {
			case drain, ok := <-d.cancelChannel:
				if ok {
					curTimer.Stop()
					if drain {
						d.egressChannel <- msg.Message
					}
					d.state = Idle
					if d.respondIdleState {
						d.idleChannel <- true
					}
				}
				return
			case <-curTimer.C:
				d.egressChannel <- msg.Message
				d.state = Idle
				if d.respondIdleState {
					d.idleChannel <- true
				}
				return
			}
		}
	}()
}

// available returns whether the delay is able to accept a new message to wait on
func (d *delay) available() bool {
	return d.state == Idle
}