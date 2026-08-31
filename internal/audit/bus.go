// Package audit ships policy decisions to a sink without blocking the request
// path. Halberd's hot loop calls bus.Record from the proxy handler; the bus
// owns a buffered channel and a single goroutine that drains it to JSONL on
// disk. Dropping on overflow is preferred to blocking the proxy.
package audit

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Event is one audit record: the decision the policy engine made about one
// JSON-RPC payload at one direction (request or response).
type Event struct {
	Time       time.Time `json:"time"`
	Direction  string    `json:"direction"`
	Method     string    `json:"method,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	Blocked    bool      `json:"blocked"`
	Violations any       `json:"violations,omitempty"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
}


// Bus is a non-blocking audit sink: callers push Events on the hot path,
// a single goroutine drains them to JSONL. Events are dropped (not blocked
// on) when the buffer fills, with a count exposed via Dropped().
//
// Bus uses a done-channel teardown rather than closing the event channel,
// so Record never risks a send-on-closed-channel panic when Stop races
// with an in-flight record.
type Bus struct {
	ch       chan Event
	done     chan struct{}
	dropped  atomic.Uint64
	sink     io.Writer
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewBus returns a running Bus that writes JSONL to sink. buf is the channel
// capacity; a zero or negative value defaults to 1024.
func NewBus(sink io.Writer, buf int) *Bus {
	if buf <= 0 {
		buf = 1024
	}
	b := &Bus{
		ch:   make(chan Event, buf),
		done: make(chan struct{}),
		sink: sink,
	}
	b.wg.Add(1)
	go b.drain()
	return b
}

// Record enqueues an event. Safe to call from any goroutine. After Stop
// has been called, or when the buffer is full, the event is counted as
// dropped and Record returns immediately.
//
// The done-channel check happens before the send attempt rather than as
// a peer in a single select. Otherwise Go's random selection between
// ready cases would let a post-Stop Record land in the still-buffered
// channel — observable to operators as a "vanished" event because the
// drain goroutine has already exited.
func (b *Bus) Record(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	select {
	case <-b.done:
		b.dropped.Add(1)
		return
	default:
	}
	select {
	case b.ch <- e:
	default:
		b.dropped.Add(1)
	}
}

// Dropped returns the cumulative count of events dropped either because
// the buffer was full or because Record was called after Stop.
func (b *Bus) Dropped() uint64 {
	return b.dropped.Load()
}

// Stop signals the drain goroutine to flush remaining buffered events and
// exit. Returns when the drain has exited or ctx is cancelled. Idempotent.
// A nil ctx is treated as context.Background() so callers can spell "wait
// forever" with bus.Stop(nil).
func (b *Bus) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	b.stopOnce.Do(func() {
		close(b.done)
	})
	waitDone := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Bus) drain() {
	defer b.wg.Done()
	enc := json.NewEncoder(b.sink)
	for {
		select {
		case <-b.done:
			// Drain whatever's already buffered, then exit. Events still
			// in flight in Record may slip past — that's acceptable; the
			// hot-path contract is best-effort, not all-or-nothing.
			for {
				select {
				case ev := <-b.ch:
					if err := enc.Encode(ev); err != nil {
						slog.Error("audit sink write failed", "error", err)
					}
				default:
					return
				}
			}
		case ev := <-b.ch:
			if err := enc.Encode(ev); err != nil {
				slog.Error("audit sink write failed", "error", err)
			}
		}
	}
}
