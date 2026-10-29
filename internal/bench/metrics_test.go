package bench

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHistogramMergeAndQuantiles(t *testing.T) {
	var a, b Histogram
	for i := 1; i <= 100; i++ {
		if i <= 50 {
			a.Observe(time.Duration(i) * time.Millisecond)
		} else {
			b.Observe(time.Duration(i) * time.Millisecond)
		}
	}
	a.Merge(b)
	s := a.Summary()
	if s.Samples != 100 || s.MinMS != 1 || s.MaxMS != 100 || s.MeanMS != 50.5 {
		t.Fatalf("bad statistics: %+v", s)
	}
	for _, pair := range [][2]float64{{s.P50MS, 50}, {s.P95MS, 95}, {s.P99MS, 99}} {
		if pair[0] < pair[1] || pair[0] > pair[1]*1.022 {
			t.Errorf("quantile outside upper-bound error: %v", pair)
		}
	}
	empty := (Histogram{}).Summary()
	if math.IsNaN(empty.MeanMS) || empty.Samples != 0 {
		t.Fatal(empty)
	}
}

func TestBudgetNeverOvershoots(t *testing.T) {
	b := budget{max: 101}
	var total atomic.Uint64
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for {
				n := b.take(7)
				if n == 0 {
					return
				}
				total.Add(n)
			}
		})
	}
	wg.Wait()
	if total.Load() != 101 {
		t.Fatal(total.Load())
	}
}

func TestSnapshotMeanIsIndependentOfElapsed(t *testing.T) {
	m := &meter{kind: "read"}
	m.record(Counts{Attempted: 1, Succeeded: 1}, 10*time.Millisecond, true)
	a := snapshot([]*meter{m}, 10*time.Second)
	if a.ByOperation["read"].Latency.MeanMS != 10 || a.OperationsPerSecond != .1 {
		t.Fatalf("wrong units %+v", a)
	}
}
