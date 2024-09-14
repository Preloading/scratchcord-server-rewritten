package main

import "sync"

type EventPublisher struct {
	subscribers   chan chan BroadcastDBMessage
	mutex         sync.Mutex
	subscriptions map[chan<- BroadcastDBMessage]<-chan BroadcastDBMessage // Map send-only to receive-only channels
}

func NewEventPublisher() *EventPublisher {
	return &EventPublisher{
		subscribers:   make(chan chan BroadcastDBMessage),
		mutex:         sync.Mutex{},
		subscriptions: make(map[chan<- BroadcastDBMessage]<-chan BroadcastDBMessage),
	}
}

func (ep *EventPublisher) Subscribe() <-chan BroadcastDBMessage {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()

	subscriber := make(chan BroadcastDBMessage)
	ep.subscriptions[subscriber] = subscriber // Store both the send and receive channels
	return subscriber
}

func (ep *EventPublisher) Unsubscribe(subscriber <-chan BroadcastDBMessage) {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()

	// Find the matching send-only channel and close it
	for s, recv := range ep.subscriptions {
		if recv == subscriber { // Compare the receive-only channel
			delete(ep.subscriptions, s)
			close(s) // Close the send-only channel
			break
		}
	}
}

func (ep *EventPublisher) Publish(data BroadcastDBMessage) {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()

	for subscriber := range ep.subscriptions {
		go func(s chan<- BroadcastDBMessage) {
			s <- data
		}(subscriber)
	}
}
