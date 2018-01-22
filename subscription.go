package redisBus

import (
	"math/rand"
)

type Subscription struct {
	DataChan chan []byte
	ExitChan chan struct{}
	bus      *RedisBus
	channel  string
	id       int
}

func (rb *RedisBus) newSubscription(channel string) *Subscription {
	return &Subscription{
		DataChan: make(chan []byte, 1),
		ExitChan: make(chan struct{}),
		bus:      rb,
		channel:  channel,
		id:       rand.Int(),
	}
}

func (s *Subscription) Close() {
	s.bus.unsubscribe(s)
	close(s.DataChan)
	close(s.ExitChan)
}
