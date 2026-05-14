package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

const bucketDuration = 5 * time.Minute

type bucketKey struct {
	backendID   string
	bucketStart time.Time
}

type bucketData struct {
	successCount int
	failureCount int
	selected     bool
}

// Aggregator accumulates health probe results and selection events into 5-minute
// buckets, flushing completed buckets to the database every 5 minutes.
type Aggregator struct {
	mu      sync.Mutex
	buckets map[bucketKey]*bucketData
	store   store.HealthHistoryStore
	wg      sync.WaitGroup
}

// NewAggregator creates a new Aggregator backed by the given store.
func NewAggregator(s store.HealthHistoryStore) *Aggregator {
	return &Aggregator{
		buckets: make(map[bucketKey]*bucketData),
		store:   s,
	}
}

// bucketFloor floors t to the nearest 5-minute boundary.
func bucketFloor(t time.Time) time.Time {
	return t.Truncate(bucketDuration)
}

// RecordProbe records a single health probe result for the given backend.
func (a *Aggregator) RecordProbe(backendID string, healthy bool) {
	key := bucketKey{backendID: backendID, bucketStart: bucketFloor(time.Now())}

	a.mu.Lock()
	defer a.mu.Unlock()

	b := a.getOrCreateBucket(key)
	if healthy {
		b.successCount++
	} else {
		b.failureCount++
	}
}

// RecordSelection marks the backend as selected (used for DNS) in the current bucket.
func (a *Aggregator) RecordSelection(backendID string) {
	key := bucketKey{backendID: backendID, bucketStart: bucketFloor(time.Now())}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.getOrCreateBucket(key).selected = true
}

func (a *Aggregator) getOrCreateBucket(key bucketKey) *bucketData {
	b := a.buckets[key]
	if b == nil {
		b = &bucketData{}
		a.buckets[key] = b
	}
	return b
}

// Start launches the background flush loop. It stops when ctx is cancelled.
func (a *Aggregator) Start(ctx context.Context) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.run(ctx)
	}()
}

// Stop waits for the background goroutine to finish.
func (a *Aggregator) Stop() {
	a.wg.Wait()
}

func (a *Aggregator) run(ctx context.Context) {
	ticker := time.NewTicker(bucketDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Flush remaining completed buckets before exit.
			a.flush(context.Background())
			return
		case <-ticker.C:
			a.flush(ctx)
		}
	}
}

// flush writes all completed buckets to the store. The current (incomplete) bucket
// is left in memory so it keeps accumulating.
func (a *Aggregator) flush(ctx context.Context) {
	currentBucket := bucketFloor(time.Now())

	a.mu.Lock()
	// Drain all buckets except the current one.
	toFlush := make(map[bucketKey]*bucketData, len(a.buckets))
	for k, v := range a.buckets {
		if k.bucketStart != currentBucket {
			toFlush[k] = v
			delete(a.buckets, k)
		}
	}
	a.mu.Unlock()

	for key, data := range toFlush {
		selected := 0
		if data.selected {
			selected = 1
		}
		bucket := &types.HealthHistoryBucket{
			BackendID:    key.backendID,
			BucketStart:  key.bucketStart,
			SuccessCount: data.successCount,
			FailureCount: data.failureCount,
			Selected:     selected,
		}
		if err := a.store.UpsertHealthHistoryBucket(ctx, bucket); err != nil {
			slog.Error("failed to flush health history bucket",
				"backend_id", key.backendID,
				"bucket_start", key.bucketStart,
				"error", err,
			)
		}
	}
}
