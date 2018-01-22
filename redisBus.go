package redisBus

import (
	"errors"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/sirupsen/logrus"
)

type RedisBus struct {
	subConn  redis.PubSubConn
	pubConn  redis.Conn
	mutex    *sync.RWMutex
	subs     map[string]map[int]*subscription
	StopChan chan struct{}
}

func New(redisURL string) (*RedisBus, error) {
	conn, err := redis.DialTimeout("tcp", redisURL, time.Second, time.Second, time.Second)
	if err != nil {
		return nil, err
	}

	subConn := redis.PubSubConn{conn}

	pubConn, err := redis.DialTimeout("tcp", redisURL, time.Second, time.Second, time.Second)
	if err != nil {
		return nil, err
	}

	rb := &RedisBus{
		subConn:  subConn,
		pubConn:  pubConn,
		mutex:    &sync.RWMutex{},
		subs:     make(map[string]map[int]*subscription),
		StopChan: make(chan struct{}),
	}

	go rb.start()

	return rb, nil
}

func (rb *RedisBus) start() {
	for {
		if rb.cancelled() {
			return
		}

		switch v := rb.subConn.Receive().(type) {
		case redis.Message:
			logrus.Debugf("%s: message: %s\n", v.Channel, v.Data)
			rb.mutex.RLock()

			for _, sub := range rb.subs[v.Channel] {
				sub.dataChan <- v.Data
			}

			rb.mutex.RUnlock()
		case redis.Subscription:
			logrus.Debugf("%s: %s %d\n", v.Channel, v.Kind, v.Count)
		case error:
			logrus.Debug("msg error:", v)
			rb.Close()
		}
	}
}

func (rb *RedisBus) Close() error {
	close(rb.StopChan)

	err := rb.pubConn.Close()
	if err != nil {
		return err
	}

	err = rb.subConn.Close()
	if err != nil {
		return err
	}

	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	for channel, subMap := range rb.subs {
		for subID, sub := range subMap {
			close(sub.exitChan)
			close(sub.dataChan)
			delete(subMap, subID)
		}
		delete(rb.subs, channel)
	}

	return nil
}

func (rb *RedisBus) Subscribe(channel string) (Subscription, error) {
	if rb.cancelled() {
		return nil, errors.New("RedisBus instance closed")
	}

	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	_, ok := rb.subs[channel]
	if !ok {
		err := rb.subConn.Subscribe(channel)
		if err != nil {
			return nil, err
		}
		rb.subs[channel] = make(map[int]*subscription)
	}

	sub := rb.newSubscription(channel)

	rb.subs[channel][sub.id] = sub

	return sub, nil
}

func (rb *RedisBus) Publish(channel string, data []byte) error {
	if rb.cancelled() {
		return errors.New("RedisBus instance closed")
	}

	_, err := rb.pubConn.Do("PUBLISH", channel, data)
	return err
}

func (rb *RedisBus) unsubscribe(sub *subscription) {
	if rb.cancelled() {
		return
	}

	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	delete(rb.subs[sub.channel], sub.id)

	if len(rb.subs[sub.channel]) == 0 {
		err := rb.subConn.Unsubscribe(sub.channel)
		logrus.Debug("unsub error:", err)
		if err != nil {
			rb.Close()
		}
	}
}

func (rb *RedisBus) cancelled() bool {
	select {
	case <-rb.StopChan:
		return true
	default:
		return false
	}
}
