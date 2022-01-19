package chain

import (
	"context"
	"errors"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/gammazero/workerpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/lily/metrics"
)

// NewWatcher creates a new Watcher. confidence sets the number of tipsets that will be held
// in a cache awaiting possible reversion. Tipsets will be written to the database when they are evicted from
// the cache due to incoming later tipsets.
func NewWatcher(obs TipSetObserver, hn HeadNotifier, cache *TipSetCache) *Watcher {
	return &Watcher{
		notifier:   hn,
		obs:        obs,
		confidence: cache.Confidence(),
		cache:      cache,
		indexSlot:  make(chan struct{}, 1), // allow one concurrent indexing job
		wp:         workerpool.New(8),      // TODO this value should be derived from the window duration passed to indexer
	}
}

// Watcher is a task that indexes blocks by following the chain head.
type Watcher struct {
	notifier   HeadNotifier
	obs        TipSetObserver
	confidence int           // size of tipset cache
	cache      *TipSetCache  // caches tipsets for possible reversion
	indexSlot  chan struct{} // filled with a token when a goroutine is indexing a tipset
	wp         *workerpool.WorkerPool
	done       chan struct{}
}

// Run starts following the chain head and blocks until the context is done or
// an error occurs.
func (c *Watcher) Run(ctx context.Context) error {
	// init the done channel for each run since jobs may be started and stopped.
	c.done = make(chan struct{})
	defer close(c.done)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case he, ok := <-c.notifier.HeadEvents():
			if !ok {
				return c.notifier.Err()
			}
			if he != nil && he.TipSet != nil {
				metrics.RecordCount(ctx, metrics.WatchHeight, int(he.TipSet.Height()))
			}

			if err := c.index(ctx, he); err != nil {
				return xerrors.Errorf("index: %w", err)
			}
		}
	}
}

func (c *Watcher) Done() <-chan struct{} {
	return c.done
}

func (c *Watcher) index(ctx context.Context, he *HeadEvent) error {
	switch he.Type {
	case HeadEventCurrent:
		err := c.cache.SetCurrent(he.TipSet)
		if err != nil {
			log.Errorw("tipset cache set current", "error", err.Error())
		}

		// If we have a zero confidence window then we need to notify every tipset we see
		if c.confidence == 0 {
			if err := c.obs.TipSet(ctx, he.TipSet); err != nil {
				return xerrors.Errorf("notify tipset: %w", err)
			}
		}
	case HeadEventApply:
		tail, err := c.cache.Add(he.TipSet)
		if err != nil {
			log.Errorw("tipset cache add", "error", err.Error())
		}

		// Send the tipset that fell out of the confidence window to the observer
		if tail != nil {
			if err := c.maybeIndexTipSet(ctx, tail); err != nil {
				return xerrors.Errorf("notify tipset: %w", err)
			}
		}

	case HeadEventRevert:
		err := c.cache.Revert(he.TipSet)
		if err != nil {
			if errors.Is(err, ErrEmptyRevert) {
				// The chain is unwinding but our cache is empty. This probably means we have already processed
				// the tipset being reverted and may process it again or an alternate heaviest tipset for this height.
				metrics.RecordInc(ctx, metrics.TipSetCacheEmptyRevert)
			}
			log.Errorw("tipset cache revert", "error", err.Error())
		}
	}

	metrics.RecordCount(ctx, metrics.TipSetCacheSize, c.cache.Size())
	metrics.RecordCount(ctx, metrics.TipSetCacheDepth, c.cache.Len())

	log.Debugw("tipset cache", "height", c.cache.Height(), "tail_height", c.cache.TailHeight(), "length", c.cache.Len())

	return nil
}

// maybeIndexTipSet is called when a new tipset has been discovered
func (c *Watcher) maybeIndexTipSet(ctx context.Context, ts *types.TipSet) error {
	ctx, span := otel.Tracer("").Start(ctx, "Watcher.maybeIndexTipSet")
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String("tipset", ts.String()),
			attribute.Int64("height", int64(ts.Height())),
		)
	}
	defer span.End()
	// Process the tipset if we can, otherwise skip it so we don't block if indexing is too slow
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// record how many workers are currently waiting to execute
		metrics.RecordCount(ctx, metrics.WatcherWaitingWorkers, c.wp.WaitingQueueSize())
		c.wp.Submit(func() {
			// record a new worker starting
			metrics.RecordInc(ctx, metrics.WatcherActiveWorkers)
			if err := c.obs.TipSet(ctx, ts); err != nil {
				log.Errorw("failed to index tipset", "error", err, "height", ts.Height())
			}
			// record a worker completing.
			metrics.RecordDec(ctx, metrics.WatcherActiveWorkers)
		})
	}

	return nil // only fatal errors should be returned
}

// A HeadNotifier reports tipset events that occur at the head of the chain
type HeadNotifier interface {
	// HeadEvents returns a channel that receives head events. It may be closed
	// by the sender of the events, in which case Err will return a non-nil error
	// explaining why. HeadEvents may return nil if this implementation will never
	// notify any events.
	HeadEvents() <-chan *HeadEvent

	// Err returns the reason for the closing of the HeadEvents channel.
	Err() error
}

// A HeadEvent is a notification of a change at the head of the chain
type HeadEvent struct {
	Type   string
	TipSet *types.TipSet
}

// Constants for HeadEvent types
const (
	// HeadEventRevert indicates that the event signals a reversion of a tipset from the chain
	HeadEventRevert = "revert"

	// HeadEventApply indicates that the event signals the application of a tipset to the chain
	HeadEventApply = "apply"

	// HeadEventCurrent indicates that the event signals the current known head tipset
	HeadEventCurrent = "current"
)
