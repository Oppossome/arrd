package torrent

import (
	"sync"
	"time"
)

const (
	// sampleInterval is how often transfer counters are read.
	sampleInterval = time.Second
	// historyLen samples of totals are kept (5 minutes at 1/s).
	historyLen = 300
	// rateSmoothing is the EWMA weight given to the newest sample.
	rateSmoothing = 0.4
)

// Rate is a smoothed transfer rate in bytes per second.
type Rate struct {
	Down float64
	Up   float64
}

// Sample is the engine-wide rate at one instant.
type Sample struct {
	At time.Time
	Rate
}

// Throughput is the engine-wide transfer rate and its recent history.
type Throughput struct {
	Rate
	// PeakDown is the highest smoothed download rate seen since start.
	PeakDown float64
	// History is oldest first, at most historyLen samples.
	History []Sample
}

// counters are a torrent's cumulative payload byte counts.
type counters struct {
	read    int64
	written int64
}

// rateTracker turns cumulative byte counters into smoothed rates.
type rateTracker struct {
	mu      sync.Mutex
	lastAt  time.Time
	last    map[string]counters
	rates   map[string]Rate
	history []Sample
	peak    float64
}

func newRateTracker() *rateTracker {
	return &rateTracker{last: map[string]counters{}, rates: map[string]Rate{}}
}

// sample records the counters of every torrent currently in the engine at
// now. Torrents absent from current are forgotten.
func (r *rateTracker) sample(now time.Time, current map[string]counters) {
	r.mu.Lock()
	defer r.mu.Unlock()

	elapsed := now.Sub(r.lastAt).Seconds()
	first := r.lastAt.IsZero() || elapsed <= 0
	var total Rate
	for hash, c := range current {
		prev, seen := r.last[hash]
		r.last[hash] = c
		if first || !seen {
			continue
		}
		// Counters restart when a torrent is re-added; a negative delta
		// is a reset, not traffic.
		instant := Rate{
			Down: max(0, float64(c.read-prev.read)) / elapsed,
			Up:   max(0, float64(c.written-prev.written)) / elapsed,
		}
		old := r.rates[hash]
		smoothed := Rate{
			Down: rateSmoothing*instant.Down + (1-rateSmoothing)*old.Down,
			Up:   rateSmoothing*instant.Up + (1-rateSmoothing)*old.Up,
		}
		r.rates[hash] = smoothed
		total.Down += smoothed.Down
		total.Up += smoothed.Up
	}
	for hash := range r.last {
		if _, ok := current[hash]; !ok {
			delete(r.last, hash)
			delete(r.rates, hash)
		}
	}
	r.lastAt = now
	if first {
		return
	}
	r.peak = max(r.peak, total.Down)
	if len(r.history) == historyLen {
		copy(r.history, r.history[1:])
		r.history = r.history[:historyLen-1]
	}
	r.history = append(r.history, Sample{At: now, Rate: total})
}

// rate returns the smoothed rate of one torrent; zero if unknown.
func (r *rateTracker) rate(infoHash string) Rate {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rates[infoHash]
}

// throughput returns the latest totals and a copy of the history.
func (r *rateTracker) throughput() Throughput {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := Throughput{PeakDown: r.peak, History: append([]Sample(nil), r.history...)}
	if n := len(r.history); n > 0 {
		t.Rate = r.history[n-1].Rate
	}
	return t
}
