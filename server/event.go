package server

import (
	"sync"

	"harnais/graph"
)

type EventBus struct {
	mu sync.RWMutex

	subscribers map[string]map[chan graph.Event]struct{}

	history map[string][]graph.Event

	nextID uint64
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(
			map[string]map[chan graph.Event]struct{},
		),

		history: make(
			map[string][]graph.Event,
		),
	}
}

// ------------------------------------------------------------
// Publish
// ------------------------------------------------------------

func (b *EventBus) Publish(
	event graph.Event,
) {

	b.mu.Lock()

	b.nextID++

	event.ID = b.nextID

	b.history[event.RunID] =
		append(
			b.history[event.RunID],
			event,
		)

	subscribers :=
		b.subscribers[event.RunID]

	channels := make(
		[]chan graph.Event,
		0,
		len(subscribers),
	)

	for channel := range subscribers {
		channels = append(
			channels,
			channel,
		)
	}

	b.mu.Unlock()

	// Never hold the lock while sending.
	for _, channel := range channels {

		select {

		case channel <- event:

		default:
			// Slow consumer.
		}
	}
}

// ------------------------------------------------------------
// Subscribe
// ------------------------------------------------------------

func (b *EventBus) Subscribe(
	runID string,
) (<-chan graph.Event, func()) {

	channel := make(
		chan graph.Event,
		256,
	)

	b.mu.Lock()

	if b.subscribers[runID] == nil {

		b.subscribers[runID] =
			make(
				map[chan graph.Event]struct{},
			)
	}

	b.subscribers[runID][channel] = struct{}{}

	b.mu.Unlock()

	unsubscribe := func() {

		b.mu.Lock()
		defer b.mu.Unlock()

		subscribers :=
			b.subscribers[runID]

		if _, exists := subscribers[channel]; !exists {
			return
		}

		delete(
			subscribers,
			channel,
		)

		close(channel)
	}

	return channel, unsubscribe
}

// ------------------------------------------------------------
// Event history
// ------------------------------------------------------------

func (b *EventBus) History(
	runID string,
) []graph.Event {

	b.mu.RLock()
	defer b.mu.RUnlock()

	events := b.history[runID]

	result := make(
		[]graph.Event,
		len(events),
	)

	copy(
		result,
		events,
	)

	return result
}
