package metrics

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Labels is a simple label set for metrics
type Labels map[string]string

// counterKey encodes metric name + labels for map keys
type counterKey string

func makeKey(name string, labels Labels) counterKey {
    if len(labels) == 0 {
        return counterKey(name)
    }
    keys := make([]string, 0, len(labels))
    for k := range labels {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    b := strings.Builder{}
    b.WriteString(name)
    b.WriteString("|")
    for i, k := range keys {
        if i > 0 {
            b.WriteString(",")
        }
        b.WriteString(k)
        b.WriteString("=")
        b.WriteString(labels[k])
    }
    return counterKey(b.String())
}

var (
    counters   sync.Map // map[counterKey]*uint64
    summaries  sync.Map // map[counterKey]*summary
)

// summary tracks count and sum (milliseconds), with min/max in ms
type summary struct {
    count uint64
    sumMs uint64
    minMs int64
    maxMs int64
    mu    sync.Mutex
}

// IncCounter increments a counter by delta (must be >= 0)
func IncCounter(name string, labels Labels, delta uint64) {
    key := makeKey(name, labels)
    v, _ := counters.LoadOrStore(key, new(uint64))
    p := v.(*uint64)
    atomic.AddUint64(p, delta)
}

// ObserveSummary records a latency or value in milliseconds
func ObserveSummary(name string, labels Labels, valueMs int64) {
    key := makeKey(name, labels)
    v, _ := summaries.LoadOrStore(key, &summary{minMs: valueMs, maxMs: valueMs})
    s := v.(*summary)
    atomic.AddUint64(&s.count, 1)
    if valueMs > 0 {
        atomic.AddUint64(&s.sumMs, uint64(valueMs))
    }
    // track min/max under lock
    s.mu.Lock()
    if valueMs < s.minMs {
        s.minMs = valueMs
    }
    if valueMs > s.maxMs {
        s.maxMs = valueMs
    }
    s.mu.Unlock()
}


