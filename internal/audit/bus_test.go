package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// syncBuf is an io.Writer safe for concurrent use, so the drain goroutine
// and test assertions can read/write without racing.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestBus_RecordsToJSONL(t *testing.T) {
	sink := &syncBuf{}
	bus := NewBus(sink, 16)

	bus.Record(Event{Direction: "request", Method: "tools/call", Tool: "query"})
	bus.Record(Event{Direction: "response", Blocked: false})

	if err := bus.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(sink.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), sink.String())
	}
	var first Event
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("decode line 1: %v", err)
	}
	if first.Direction != "request" || first.Tool != "query" {
		t.Errorf("line 1 = %+v", first)
	}
}

func TestBus_StampsTimeWhenZero(t *testing.T) {
	sink := &syncBuf{}
	bus := NewBus(sink, 4)
	before := time.Now().UTC()
	bus.Record(Event{Direction: "request"})
	_ = bus.Stop(context.Background())
	after := time.Now().UTC()

	var got Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(sink.String())), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Time.Before(before) || got.Time.After(after) {
		t.Errorf("Time = %v, not in [%v, %v]", got.Time, before, after)
	}
}

func TestBus_PreservesProvidedTime(t *testing.T) {
	sink := &syncBuf{}
	bus := NewBus(sink, 4)
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	bus.Record(Event{Direction: "request", Time: want})
	_ = bus.Stop(context.Background())

	var got Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(sink.String())), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Time.Equal(want) {
		t.Errorf("Time = %v, want %v", got.Time, want)
	}
}

func TestBus_DropsWhenFull(t *testing.T) {
	// Sink that blocks until released — fills the channel, every
	// subsequent Record must hit the default branch and count as dropped.
	w := newGate()
	bus := NewBus(w, 2)
	defer func() {
		w.release()
		_ = bus.Stop(context.Background())
	}()

	for i := 0; i < 100; i++ {
		bus.Record(Event{Direction: "request"})
	}
	if dropped := bus.Dropped(); dropped == 0 {
		t.Error("expected drops when sink blocks and buffer fills, got 0")
	}
}

func TestBus_RecordAfterStopIsDropped(t *testing.T) {
	sink := &syncBuf{}
	bus := NewBus(sink, 16)
	if err := bus.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	bus.Record(Event{Direction: "request"})
	if bus.Dropped() == 0 {
		t.Error("expected drop on post-stop Record")
	}
	if strings.Contains(sink.String(), "request") {
		t.Error("post-stop Record reached the sink")
	}
}

func TestBus_StopIsIdempotent(t *testing.T) {
	sink := &syncBuf{}
	bus := NewBus(sink, 4)
	if err := bus.Stop(context.Background()); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if err := bus.Stop(context.Background()); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

func TestBus_StopHonoursContextDeadline(t *testing.T) {
	w := newGate()
	defer w.release()
	bus := NewBus(w, 0)
	// Fill the channel so the drain goroutine is stuck on Write.
	for i := 0; i < 1024; i++ {
		bus.Record(Event{Direction: "request"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := bus.Stop(ctx)
	if err == nil {
		t.Fatal("expected ctx.DeadlineExceeded when sink is blocked, got nil")
	}
}

func TestBus_StopNilCtxTreatedAsBackground(t *testing.T) {
	sink := &syncBuf{}
	bus := NewBus(sink, 4)
	if err := bus.Stop(nil); err != nil { //nolint:staticcheck // explicit nil is the test
		t.Fatalf("nil ctx panic-or-error: %v", err)
	}
}

func TestBus_ConcurrentRecordersAreSafe(t *testing.T) {
	sink := &syncBuf{}
	bus := NewBus(sink, 1024)

	const goroutines = 16
	const perRoutine = 256
	var wg sync.WaitGroup
	var sent atomic.Uint64
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perRoutine; i++ {
				bus.Record(Event{Direction: "request"})
				sent.Add(1)
			}
		}()
	}
	wg.Wait()
	if err := bus.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	written := uint64(strings.Count(sink.String(), "\n")) //nolint:gosec // strings.Count is non-negative
	dropped := bus.Dropped()
	if written+dropped != sent.Load() {
		t.Errorf("written=%d + dropped=%d != sent=%d (events vanished)",
			written, dropped, sent.Load())
	}
}

// gate is an io.Writer whose Write blocks until release() is called.
// Tests use it to keep the drain goroutine stuck on Write long enough to
// observe back-pressure on the channel, then release it so cleanup can
// proceed.
type gate struct {
	ch chan struct{}
}

func newGate() *gate {
	return &gate{ch: make(chan struct{})}
}

func (g *gate) Write(p []byte) (int, error) {
	<-g.ch
	return len(p), nil
}

func (g *gate) release() {
	// Close is the broadcast unblock; idempotent under sync.Once-like guard.
	select {
	case <-g.ch:
	default:
		close(g.ch)
	}
}

type errWriter struct {
	err error
}

func (e *errWriter) Write(_ []byte) (int, error) {
	return 0, e.err
}

func TestBus_SinkError(_ *testing.T) {
	// Normal drain error path
	ew := &errWriter{err: errors.New("write failed")}
	bus := NewBus(ew, 4)
	bus.Record(Event{Direction: "request"})
	time.Sleep(10 * time.Millisecond)
	_ = bus.Stop(context.Background())

	// Stop drain error path (done already closed with buffered events)
	b := &Bus{
		ch:   make(chan Event, 8),
		done: make(chan struct{}),
		sink: &errWriter{err: errors.New("drain write failed")},
	}
	for i := 0; i < 5; i++ {
		b.ch <- Event{Direction: "req"}
	}
	close(b.done)
	b.wg.Add(1)
	b.drain()
}
