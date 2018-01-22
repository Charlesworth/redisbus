package redisBus

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
)

// Bus contains the connection to Redis and supplies the Publish
// and Subscribe methods to access the PubSub features. ExitChan
// will be closed when the subscription has ended. Call Close to
// end the subscription manually.
type Bus interface {
	Publish(channel string, data []byte) error
	Subscribe(channel string) (Subscription, error)
	Close() error
	ExitChan() <-chan struct{}
}

type redisBus struct {
	subConn redis.PubSubConn
	pubConn redis.Conn
	mutex   *sync.RWMutex
	subs    map[string]map[int]*subscription
	// TODO maybe stopchan should return an error
	stopChan chan struct{}
	logger   *log.Logger
}

// New initialises and starts a redisbus Bus instance
func New(redisURL string) (Bus, error) {
	return new(redisURL)
}

// NewWithLogger initialises and starts a redisbus Bus instance
// with a logger
func NewWithLogger(redisURL string, logger *log.Logger) (Bus, error) {
	bus, err := new(redisURL)
	bus.logger = logger
	return bus, err
}

func new(redisURL string) (*redisBus, error) {
	conn, err := redis.DialTimeout("tcp", redisURL, time.Second, time.Second, time.Second)
	if err != nil {
		return nil, err
	}

	subConn := redis.PubSubConn{Conn: conn}

	pubConn, err := redis.DialTimeout("tcp", redisURL, time.Second, time.Second, time.Second)
	if err != nil {
		return nil, err
	}

	rb := &redisBus{
		subConn:  subConn,
		pubConn:  pubConn,
		mutex:    &sync.RWMutex{},
		subs:     make(map[string]map[int]*subscription),
		stopChan: make(chan struct{}),
	}

	go rb.start()

	return rb, nil
}

func (rb *redisBus) ExitChan() <-chan struct{} {
	return rb.stopChan
}

func (rb *redisBus) Close() error {
	close(rb.stopChan)

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

func (rb *redisBus) Subscribe(channel string) (Subscription, error) {
	if rb.cancelled() {
		return nil, errors.New("redisBus instance closed")
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

	sub := newSubscription(channel, rb)

	rb.subs[channel][sub.id] = sub

	return sub, nil
}

func (rb *redisBus) Publish(channel string, data []byte) error {
	if rb.cancelled() {
		return errors.New("redisBus instance closed")
	}

	_, err := rb.pubConn.Do("PUBLISH", channel, data)
	return err
}

func (rb *redisBus) unsubscribe(sub *subscription) {
	if rb.cancelled() {
		return
	}

	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	delete(rb.subs[sub.channel], sub.id)

	if len(rb.subs[sub.channel]) == 0 {
		err := rb.subConn.Unsubscribe(sub.channel)
		if err != nil {
			rb.Close()
		}
	}
}

func (rb *redisBus) cancelled() bool {
	select {
	case <-rb.stopChan:
		return true
	default:
		return false
	}
}

func (rb *redisBus) start() {
	for {
		if rb.cancelled() {
			return
		}

		switch v := rb.subConn.Receive().(type) {
		case redis.Message:
			if rb.logger != nil {
				rb.logger.Printf("%s: message: %s\n", v.Channel, v.Data)
			}
			rb.mutex.RLock()

			for _, sub := range rb.subs[v.Channel] {
				sub.dataChan <- v.Data
			}

			rb.mutex.RUnlock()
		case redis.Subscription:
			if rb.logger != nil {
				rb.logger.Printf("%s: %s %d\n", v.Channel, v.Kind, v.Count)
			}
		case error:
			if rb.logger != nil {
				rb.logger.Println("error:", v.Error())
			}
			rb.Close()
		}
	}
}
