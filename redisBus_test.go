package redisBus

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRedisBus(t *testing.T) {

	bus, err := New(":6379", time.Second)
	assert.NoError(t, err, "Cannot connect to redis on :6379")

	channelName := "testchannel"

	subs := []Subscription{}
	for i := 0; i < 3; i++ {
		sub, err := bus.Subscribe(channelName)
		assert.NoError(t, err, "Subscription error")
		subs = append(subs, sub)
	}

	testString := []byte("Hello Subscribers")
	for i := 0; i < 3; i++ {
		err := bus.Publish(channelName, testString)
		assert.NoError(t, err, "Publish error")

		for _, sub := range subs {
			msg := <-sub.DataChan()
			assert.Equal(t, testString, msg, "Published message and recieved message are not equal")
		}
	}

	bus.Close()
	assert.NoError(t, <-bus.ExitChan(), "Error after bus.Close()")

}
