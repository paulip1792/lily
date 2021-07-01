package metrics

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var defaultMillisecondsDistribution = view.Distribution(0.01, 0.05, 0.1, 0.3, 0.6, 0.8, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 30000, 50000, 100000, 200000, 500000, 1000000, 2000000, 5000000, 10000000, 10000000)

var (
	TaskType, _  = tag.NewKey("task")  // name of task processor
	Job, _       = tag.NewKey("job")   // name of job
	Name, _      = tag.NewKey("name")  // name of running instance of visor
	Table, _     = tag.NewKey("table") // name of table data is persisted for
	ConnState, _ = tag.NewKey("conn_state")
	API, _       = tag.NewKey("api")        // name of method on lotus api
	ActorCode, _ = tag.NewKey("actor_code") // human readable code of actor being processed

)

var (
	ProcessingDuration  = stats.Float64("processing_duration_ms", "Time taken to process a single item", stats.UnitMilliseconds)
	PersistDuration     = stats.Float64("persist_duration_ms", "Duration of a models persist operation", stats.UnitMilliseconds)
	PersistModel        = stats.Int64("persist_model", "Number of models persisted", stats.UnitDimensionless)
	DBConns             = stats.Int64("db_conns", "Database connections held", stats.UnitDimensionless)
	LensRequestDuration = stats.Float64("lens_request_duration_ms", "Duration of lotus api requets", stats.UnitMilliseconds)
	TipsetHeight        = stats.Int64("tipset_height", "The height of the tipset being processed by a task", stats.UnitDimensionless)
	ProcessingFailure   = stats.Int64("processing_failure", "Number of processing failures", stats.UnitDimensionless)
	PersistFailure      = stats.Int64("persist_failure", "Number of persistence failures", stats.UnitDimensionless)
	WatchHeight         = stats.Int64("watch_height", "The height of the tipset last seen by the watch command", stats.UnitDimensionless)
	TipSetSkip          = stats.Int64("tipset_skip", "Number of tipsets that were not processed. This is is an indication that visor cannot keep up with chain.", stats.UnitDimensionless)
	JobStart            = stats.Int64("job_start", "Number of jobs started", stats.UnitDimensionless)
	JobComplete         = stats.Int64("job_complete", "Number of jobs completed without error", stats.UnitDimensionless)
	JobError            = stats.Int64("job_error", "Number of jobs stopped due to a fatal error", stats.UnitDimensionless)
	JobTimeout          = stats.Int64("job_timeout", "Number of jobs stopped due to taking longer than expected", stats.UnitDimensionless)
)

var (
	ProcessingDurationView = &view.View{
		Measure:     ProcessingDuration,
		Aggregation: defaultMillisecondsDistribution,
		TagKeys:     []tag.Key{TaskType, ActorCode},
	}
	PersistDurationView = &view.View{
		Measure:     PersistDuration,
		Aggregation: defaultMillisecondsDistribution,
		TagKeys:     []tag.Key{TaskType, Table, ActorCode},
	}
	DBConnsView = &view.View{
		Measure:     DBConns,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{ConnState},
	}
	LensRequestDurationView = &view.View{
		Measure:     LensRequestDuration,
		Aggregation: defaultMillisecondsDistribution,
		TagKeys:     []tag.Key{TaskType, API, ActorCode},
	}
	LensRequestTotal = &view.View{
		Name:        "lens_request_total",
		Measure:     LensRequestDuration,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{TaskType, API, ActorCode},
	}
	TipsetHeightView = &view.View{
		Measure:     TipsetHeight,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{TaskType},
	}
	ProcessingFailureTotalView = &view.View{
		Name:        ProcessingFailure.Name() + "_total",
		Measure:     ProcessingFailure,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{TaskType, ActorCode},
	}
	PersistFailureTotalView = &view.View{
		Name:        PersistFailure.Name() + "_total",
		Measure:     PersistFailure,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{TaskType, Table, ActorCode},
	}
	WatchHeightView = &view.View{
		Measure:     WatchHeight,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{},
	}
	TipSetSkipTotalView = &view.View{
		Name:        TipSetSkip.Name() + "_total",
		Measure:     TipSetSkip,
		Aggregation: view.Sum(),
		TagKeys:     []tag.Key{},
	}

	JobStartTotalView = &view.View{
		Name:        JobStart.Name() + "_total",
		Measure:     JobStart,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{Job},
	}
	JobCompleteTotalView = &view.View{
		Name:        JobComplete.Name() + "_total",
		Measure:     JobComplete,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{Job},
	}
	JobErrorTotalView = &view.View{
		Name:        JobError.Name() + "_total",
		Measure:     JobError,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{Job},
	}
	JobTimeoutTotalView = &view.View{
		Name:        JobTimeout.Name() + "_total",
		Measure:     JobTimeout,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{Job},
	}

	PersistModelTotalView = &view.View{
		Name:        PersistModel.Name() + "_total",
		Measure:     PersistModel,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{TaskType, Table},
	}
)

var DefaultViews = []*view.View{
	ProcessingDurationView,
	PersistDurationView,
	DBConnsView,
	LensRequestDurationView,
	LensRequestTotal,
	TipsetHeightView,
	ProcessingFailureTotalView,
	PersistFailureTotalView,
	TipSetSkipTotalView,
	JobStartTotalView,
	JobCompleteTotalView,
	JobErrorTotalView,
	JobTimeoutTotalView,
	PersistModelTotalView,
}

// SinceInMilliseconds returns the duration of time since the provide time as a float64.
func SinceInMilliseconds(startTime time.Time) float64 {
	return float64(time.Since(startTime).Nanoseconds()) / 1e6
}

// Timer is a function stopwatch, calling it starts the timer,
// calling the returned function will record the duration.
func Timer(ctx context.Context, m *stats.Float64Measure) func() {
	start := time.Now()
	return func() {
		stats.Record(ctx, m.M(SinceInMilliseconds(start)))
	}
}

// RecordInc is a convenience function that increments a counter.
func RecordInc(ctx context.Context, m *stats.Int64Measure) {
	stats.Record(ctx, m.M(1))
}

// RecordCount is a convenience function that increments a counter by a count.
func RecordCount(ctx context.Context, m *stats.Int64Measure, count int) {
	stats.Record(ctx, m.M(int64(count)))
}

// WithTagValue is a convenience function that upserts the tag value in the given context.
func WithTagValue(ctx context.Context, k tag.Key, v string) context.Context {
	ctx, _ = tag.New(ctx, tag.Upsert(k, v))
	return ctx
}
