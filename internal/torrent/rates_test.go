package torrent

import (
	"math"
	"testing"
	"time"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestRateTrackerSmoothsAndTotals(t *testing.T) {
	r := newRateTracker()
	t0 := time.Unix(1000, 0)

	r.sample(t0, map[string]counters{"a": {}, "b": {}})
	if got := r.throughput(); len(got.History) != 0 {
		t.Fatalf("first sample is a baseline, history = %v", got.History)
	}

	r.sample(t0.Add(time.Second), map[string]counters{
		"a": {read: 1000, written: 100},
		"b": {read: 500},
	})
	a := r.rate("a")
	if !approx(a.Down, rateSmoothing*1000) || !approx(a.Up, rateSmoothing*100) {
		t.Errorf("rate a = %+v", a)
	}
	tp := r.throughput()
	if !approx(tp.Down, rateSmoothing*1500) || len(tp.History) != 1 {
		t.Errorf("throughput = %+v", tp)
	}
	if !approx(tp.PeakDown, tp.Down) {
		t.Errorf("peak = %v, want %v", tp.PeakDown, tp.Down)
	}

	// Idle second: rates decay, the peak stays.
	r.sample(t0.Add(2*time.Second), map[string]counters{
		"a": {read: 1000, written: 100},
		"b": {read: 500},
	})
	if got := r.rate("a").Down; !approx(got, (1-rateSmoothing)*rateSmoothing*1000) {
		t.Errorf("decayed rate a = %v", got)
	}
	if got := r.throughput(); !approx(got.PeakDown, rateSmoothing*1500) {
		t.Errorf("peak after idle = %v", got.PeakDown)
	}
}

func TestRateTrackerForgetsRemovedAndIgnoresResets(t *testing.T) {
	r := newRateTracker()
	t0 := time.Unix(1000, 0)
	r.sample(t0, map[string]counters{"a": {read: 5000}})
	r.sample(t0.Add(time.Second), map[string]counters{"a": {read: 6000}})
	if r.rate("a").Down == 0 {
		t.Fatal("expected a rate for a")
	}

	// Removed: forgotten.
	r.sample(t0.Add(2*time.Second), map[string]counters{})
	if got := r.rate("a"); got != (Rate{}) {
		t.Errorf("removed torrent rate = %+v", got)
	}

	// Re-added with counters starting over: the drop is not traffic.
	r.sample(t0.Add(3*time.Second), map[string]counters{"a": {read: 10}})
	r.sample(t0.Add(4*time.Second), map[string]counters{"a": {read: 0}})
	if got := r.rate("a"); got.Down != 0 {
		t.Errorf("rate after counter reset = %+v", got)
	}
}

func TestRateTrackerHistoryIsBounded(t *testing.T) {
	r := newRateTracker()
	t0 := time.Unix(1000, 0)
	for i := range historyLen + 10 {
		r.sample(t0.Add(time.Duration(i)*time.Second), map[string]counters{"a": {read: int64(i) * 100}})
	}
	h := r.throughput().History
	if len(h) != historyLen {
		t.Fatalf("history length = %d, want %d", len(h), historyLen)
	}
	if want := t0.Add(time.Duration(historyLen+9) * time.Second); !h[len(h)-1].At.Equal(want) {
		t.Errorf("newest sample at %v, want %v", h[len(h)-1].At, want)
	}
}

func TestETA(t *testing.T) {
	base := Progress{HaveInfo: true, BytesTotal: 1000, BytesCompleted: 400, DownloadRate: 100}
	if got := eta(base); got != 6*time.Second {
		t.Errorf("eta = %v, want 6s", got)
	}
	for name, p := range map[string]Progress{
		"no info":  {BytesTotal: 1000, DownloadRate: 100},
		"complete": {HaveInfo: true, Complete: true, BytesTotal: 1000, BytesCompleted: 1000, DownloadRate: 100},
		"paused":   {HaveInfo: true, Paused: true, BytesTotal: 1000, DownloadRate: 100},
		"stalled":  {HaveInfo: true, BytesTotal: 1000},
	} {
		if got := eta(p); got != 0 {
			t.Errorf("%s: eta = %v, want 0", name, got)
		}
	}
}
