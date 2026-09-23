package outbox

import (
	"context"
	"fmt"
	"strings"
)

// TraceparentHeader is the canonical W3C trace context header key.
const TraceparentHeader = "traceparent"

// TraceContext holds parsed W3C traceparent data.
type TraceContext struct {
	Version string
	TraceID string
	ParentID string
	TraceFlags string
}

// Format returns W3C traceparent formatted string: 00-{trace_id}-{parent_id}-{flags}.
func (t TraceContext) Format() string {
	return fmt.Sprintf("%s-%s-%s-%s", t.Version, t.TraceID, t.ParentID, t.TraceFlags)
}

// ParseTraceparent parses W3C traceparent header string.
func ParseTraceparent(raw string) (TraceContext, error) {
	parts := strings.Split(raw, "-")
	if len(parts) != 4 {
		return TraceContext{}, fmt.Errorf("invalid traceparent format: %s", raw)
	}
	if len(parts[0]) != 2 || len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return TraceContext{}, fmt.Errorf("invalid traceparent field lengths: %s", raw)
	}
	return TraceContext{
		Version:    parts[0],
		TraceID:    parts[1],
		ParentID:   parts[2],
		TraceFlags: parts[3],
	}, nil
}

// InjectTraceparent injects trace context into event metadata.
func InjectTraceparent(ctx context.Context, headers map[string]string, trace string) {
	if headers != nil {
		headers[TraceparentHeader] = trace
	}
}
