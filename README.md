[![Go Report Card](https://goreportcard.com/badge/github.com/Charlesworth/redisbus)](https://goreportcard.com/report/github.com/Charlesworth/redisbus)
[![GoDoc](https://godoc.org/github.com/Charlesworth/redisbus?status.svg)](https://godoc.org/github.com/Charlesworth/redisbus)
# redisbus
A pubsub bus built on top of Redis. This library will multiplex Redis subscriptions to a larger number of local subscriptions. 

To get started:

    $ go get github.com/charlesworth/redisbus

## Interface
```
func New(redisURL string, dialTimeout time.Duration) (Bus, error)
func NewWithLogger(redisURL string, dialTimeout time.Duration, logger *log.Logger) (Bus, error)

type Bus interface {
    Publish(channel string, data []byte) error
    Subscribe(channel string) (Subscription, error)
    ExitChan() <-chan error
    Close()
}

type Subscription interface {
    DataChan() <-chan []byte
    ExitChan() <-chan struct{}
    Close()
}
```


## Example
```golang
package main

import (
    "fmt"
    "github.com/charlesworth/redisbus"
    "time"
)

func main() {
    bus, _ := redisbus.New(":6379", time.Second)

    channelName := "chat"
    subscriber, _ := bus.Subscribe(channelName)

    message := []byte("Hello Subscribers")
    bus.Publish(channelName, message)

    fmt.Printf("%s\n", <-subscriber.DataChan())
}
```

## Testing
Requires a running Redis instance on port :6379

    $ go test