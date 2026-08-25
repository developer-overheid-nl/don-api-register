package jobs

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

const (
	refreshHour   = 7
	refreshMinute = 0
	runTimeout    = 120 * time.Minute
	refreshPeriod = 24 * time.Hour
)

type OASRefresher interface {
	RefreshChangedApis(ctx context.Context) (models.OASRefreshResult, error)
}

// OASRefreshJob draait direct na startup en daarna dagelijks om 07:00 een refresh-run.
type OASRefreshJob struct {
	refresher OASRefresher
	location  *time.Location
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewOASRefreshJob start direct een refresh-run en plant daarna een dagelijkse job. Parent context kan nil zijn.
func NewOASRefreshJob(refresher OASRefresher, parentCtx context.Context) *OASRefreshJob {
	if refresher == nil {
		return nil
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	job := &OASRefreshJob{
		refresher: refresher,
		location:  time.Local,
		ctx:       ctx,
		cancel:    cancel,
	}
	go func() {
		job.runOnce()
		job.loop()
	}()
	return job
}

// Stop beëindigt de job.
func (j *OASRefreshJob) Stop() {
	if j == nil || j.cancel == nil {
		return
	}
	j.cancel()
}

func (j *OASRefreshJob) loop() {
	for {
		delay := time.Until(nextRunAt(time.Now().In(j.location), refreshHour, refreshMinute))
		timer := time.NewTimer(delay)
		select {
		case <-j.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			j.runOnce()
		}
	}
}

func (j *OASRefreshJob) runOnce() {
	startedAt := time.Now()
	runCtx, cancel := context.WithTimeout(j.ctx, runTimeout)
	defer cancel()

	result, err := j.refresher.RefreshChangedApis(runCtx)
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.DebugContext(
				runCtx,
				"OAS refresh canceled",
				"component", "oas_refresh",
				"operation", "run",
				"candidate_count", result.CandidateCount,
				"processed_count", result.ProcessedCount,
				"updated_count", result.UpdatedCount,
				"unavailable_count", result.UnavailableCount,
				"failed_count", result.FailedCount,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"heap_alloc_bytes", memory.Alloc,
			)
		} else {
			slog.ErrorContext(
				runCtx,
				"OAS refresh failed",
				"component", "oas_refresh",
				"operation", "run",
				"candidate_count", result.CandidateCount,
				"processed_count", result.ProcessedCount,
				"updated_count", result.UpdatedCount,
				"unavailable_count", result.UnavailableCount,
				"failed_count", result.FailedCount,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"heap_alloc_bytes", memory.Alloc,
				"error", err,
			)
		}
		return
	}
	slog.InfoContext(
		runCtx,
		"OAS refresh completed",
		"component", "oas_refresh",
		"operation", "run",
		"candidate_count", result.CandidateCount,
		"processed_count", result.ProcessedCount,
		"updated_count", result.UpdatedCount,
		"unavailable_count", result.UnavailableCount,
		"failed_count", result.FailedCount,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"heap_alloc_bytes", memory.Alloc,
	)
}

func nextRunAt(now time.Time, hour, minute int) time.Time {
	loc := now.Location()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(now) {
		candidate = candidate.Add(refreshPeriod)
	}
	return candidate
}
