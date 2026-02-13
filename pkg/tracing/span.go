package tracing

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type contextKey string

const spanKey contextKey = "trace_span"

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

func (s *Span) End() {
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
}

func (s *Span) SetAttr(key string, value any) {
	s.mu.Lock()
	s.Attrs[key] = value
	s.mu.Unlock()
}

func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanKey).(*Span); ok {
		return span
	}
	return nil
}

func (s *Span) Log() {
	s.logRecursive(0)
}

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
