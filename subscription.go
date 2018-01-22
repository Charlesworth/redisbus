package redisBus

import (
	"math/rand"
)

// Subscription is an object that provides subscription data
// over the DataChan. ExitChan will be closed when the subscription
// has ended. Call Close to end the subscription manually.
type Subscription interface {
	DataChan() <-chan []byte
	ExitChan() <-chan struct{}
	Close()
}

type subscription struct {
	dataChan chan []byte
	exitChan chan struct{}
	bus      *RedisBus
	channel  string
	id       int
}

func (rb *RedisBus) newSubscription(channel string) *subscription {
	return &subscription{
		dataChan: make(chan []byte, 1),
		exitChan: make(chan struct{}),
		bus:      rb,
		channel:  channel,
		id:       rand.Int(),
	}
}

func (s *subscription) ExitChan() <-chan struct{} {
	return s.exitChan
}

func (s *subscription) DataChan() <-chan []byte {
	return s.dataChan
}

func (s *subscription) Close() {
	s.bus.unsubscribe(s)
	close(s.dataChan)
	close(s.exitChan)
}
