package bench

import (
	"math"
	"sync"
	"time"
)

// Histogram uses bounded logarithmic buckets with at most 2.2% upper-bound
// quantile error. It never retains individual samples or grows with run length.
type Histogram struct {
	Bins     [2048]uint64
	Count    uint64
	Sum      float64
	Min, Max float64
}

func (h *Histogram) Observe(d time.Duration) {
	ns := float64(max(d.Nanoseconds(), 1))
	i := min(2047, int(math.Ceil(math.Log2(ns)*32)))
	h.Bins[i]++
	h.Count++
	h.Sum += ns
	if h.Count == 1 || ns < h.Min {
		h.Min = ns
	}
	h.Max = max(h.Max, ns)
}
func (h *Histogram) Merge(other Histogram) {
	if other.Count == 0 {
		return
	}
	if h.Count == 0 || other.Min < h.Min {
		h.Min = other.Min
	}
	h.Max = max(h.Max, other.Max)
	h.Count += other.Count
	h.Sum += other.Sum
	for i, n := range other.Bins {
		h.Bins[i] += n
	}
}
func (h Histogram) quantile(q float64) float64 {
	target := uint64(math.Ceil(float64(h.Count) * q))
	var n uint64
	for i, x := range h.Bins {
		n += x
		if n >= target && target > 0 {
			return min(h.Max, math.Exp2(float64(i)/32)) / 1e6
		}
	}
	return 0
}

// Latency reports successful-unit latency in milliseconds.
type Latency struct {
	Samples uint64  `json:"samples"`
	MinMS   float64 `json:"min_ms"`
	MeanMS  float64 `json:"mean_ms"`
	MaxMS   float64 `json:"max_ms"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
	P99MS   float64 `json:"p99_ms"`
}

func (h Histogram) Summary() Latency {
	if h.Count == 0 {
		return Latency{}
	}
	return Latency{h.Count, h.Min / 1e6, h.Sum / float64(h.Count) / 1e6, h.Max / 1e6, h.quantile(.5), h.quantile(.95), h.quantile(.99)}
}

// Counts distinguishes attempted SQL from acknowledged successful work.
type Counts struct {
	Attempted    uint64 `json:"sql_attempted"`
	Succeeded    uint64 `json:"sql_succeeded"`
	Failed       uint64 `json:"sql_failed"`
	Canceled     uint64 `json:"sql_canceled"`
	RolledBack   uint64 `json:"sql_rolled_back"`
	Unknown      uint64 `json:"sql_commit_unknown"`
	Rows         uint64 `json:"rows"`
	Transactions uint64 `json:"transactions_committed"`
}

func (c *Counts) add(o Counts) {
	c.Attempted += o.Attempted
	c.Succeeded += o.Succeeded
	c.Failed += o.Failed
	c.Canceled += o.Canceled
	c.RolledBack += o.RolledBack
	c.Unknown += o.Unknown
	c.Rows += o.Rows
	c.Transactions += o.Transactions
}

type meter struct {
	mu      sync.Mutex
	counts  Counts
	latency Histogram
	kind    string
}

func (m *meter) record(c Counts, latency time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts.add(c)
	if success {
		m.latency.Observe(latency)
	}
}

// Operation groups counters and successful-unit latency for one operation kind.
type Operation struct {
	Counts
	Latency Latency `json:"unit_latency"`
}

// Snapshot is a concurrency-safe view of cumulative workload progress.
type Snapshot struct {
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Counts
	OperationsPerSecond   float64              `json:"operations_per_second"`
	TransactionsPerSecond float64              `json:"transactions_per_second"`
	ByOperation           map[string]Operation `json:"by_operation"`
	Interval              *Interval            `json:"interval,omitempty"`
}

// Interval contains rates since the preceding progress snapshot.
type Interval struct {
	Seconds               float64 `json:"seconds"`
	OperationsPerSecond   float64 `json:"operations_per_second"`
	TransactionsPerSecond float64 `json:"transactions_per_second"`
}

func snapshot(meters []*meter, elapsed time.Duration) Snapshot {
	s := Snapshot{ElapsedSeconds: elapsed.Seconds(), ByOperation: map[string]Operation{}}
	hist := map[string]Histogram{}
	for _, m := range meters {
		m.mu.Lock()
		counts, h := m.counts, m.latency
		m.mu.Unlock()
		s.Counts.add(counts)
		op := s.ByOperation[m.kind]
		op.Counts.add(counts)
		s.ByOperation[m.kind] = op
		merged := hist[m.kind]
		merged.Merge(h)
		hist[m.kind] = merged
	}
	for kind, op := range s.ByOperation {
		op.Latency = hist[kind].Summary()
		s.ByOperation[kind] = op
	}
	if s.ElapsedSeconds > 0 {
		s.OperationsPerSecond = float64(s.Succeeded) / s.ElapsedSeconds
		s.TransactionsPerSecond = float64(s.Transactions) / s.ElapsedSeconds
	}
	return s
}
