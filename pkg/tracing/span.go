// Package tracing provides a lightweight span-based tracing system that
// propagates trace context through Go contexts. Spans form parent–child trees
// and are logged as structured JSON via slog.
package tracing

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type contextKey string

const spanKey contextKey = "trace_span"

// Span represents a timed operation within a trace.
type Span struct {
	Name      string
	TraceID   string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Children  []*Span
	Attrs     map[string]any
	mu        sync.Mutex
}

// StartSpan creates a new root span and stores it in the returned context.
func StartSpan(ctx context.Context, name string, traceID string) (context.Context, *Span) {
	span := &Span{
		Name:      name,
		TraceID:   traceID,
		StartTime: time.Now(),
		Children:  make([]*Span, 0),
		Attrs:     make(map[string]any),
	}
	return context.WithValue(ctx, spanKey, span), span
}

// StartChildSpan creates a child span linked to the parent in ctx.
func StartChildSpan(ctx context.Context, name string) (context.Context, *Span) {
	parent := SpanFromContext(ctx)
	child := &Span{
		Name:      name,
		StartTime: time.Now(),
		Children:  make([]*Span, 0),
		Attrs:     make(map[string]any),
	}

	if parent != nil {
		child.TraceID = parent.TraceID
		parent.mu.Lock()
		parent.Children = append(parent.Children, child)
		parent.mu.Unlock()
	}

	return context.WithValue(ctx, spanKey, child), child
}

// End records the span's end time and duration.
func (s *Span) End() {
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
}

// SetAttr attaches a key-value attribute to the span.
func (s *Span) SetAttr(key string, value any) {
	s.mu.Lock()
	s.Attrs[key] = value
	s.mu.Unlock()
}

// SpanFromContext extracts the current Span from ctx, or nil if none.
func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanKey).(*Span); ok {
		return span
	}
	return nil
}

// Log writes the span tree to slog.
func (s *Span) Log() {
	s.logRecursive(0)
}

// logRecursive recursively logs spans with increasing depth.
func (s *Span) logRecursive(depth int) {
	attrs := []any{
		"trace_id", s.TraceID,
		"span", s.Name,
		"duration_ms", s.Duration.Milliseconds(),
		"depth", depth,
	}
	for k, v := range s.Attrs {
		attrs = append(attrs, k, v)
	}
	slog.Info("span", attrs...)

	for _, child := range s.Children {
		child.logRecursive(depth + 1)
	}
}
