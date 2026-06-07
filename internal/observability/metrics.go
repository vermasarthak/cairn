package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type Metrics struct {
	mu       sync.Mutex
	counters map[string]uint64
}

func NewMetrics() *Metrics { return &Metrics{counters: map[string]uint64{}} }

func (m *Metrics) Inc(name string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.counters[name]++
	m.mu.Unlock()
}

func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		m.mu.Lock()
		names := make([]string, 0, len(m.counters))
		for name := range m.counters {
			names = append(names, name)
		}
		sort.Strings(names)
		lines := make([]string, 0, len(names))
		for _, name := range names {
			lines = append(lines, fmt.Sprintf("%s %d", name, m.counters[name]))
		}
		m.mu.Unlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
	}
}
