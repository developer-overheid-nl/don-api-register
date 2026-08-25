package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commonlogging "github.com/developer-overheid-nl/don-register-common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextRunAt_BeforeTarget(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2025, 4, 10, 6, 15, 0, 0, loc)
	next := nextRunAt(now, refreshHour, refreshMinute)
	expected := time.Date(2025, 4, 10, 7, 0, 0, 0, loc)
	assert.Equal(t, expected, next)
}

func TestNextRunAt_AfterTarget(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2025, 4, 10, 13, 0, 0, 0, loc)
	next := nextRunAt(now, refreshHour, refreshMinute)
	expected := time.Date(2025, 4, 11, 7, 0, 0, 0, loc)
	assert.Equal(t, expected, next)
}

type refreshStub struct {
	result     models.OASRefreshResult
	err        error
	called     chan context.Context
	cancelSeen bool
}

func (s *refreshStub) RefreshChangedApis(ctx context.Context) (models.OASRefreshResult, error) {
	if s.called != nil {
		s.called <- ctx
	}
	if ctx.Err() != nil {
		s.cancelSeen = true
	}
	return s.result, s.err
}

func TestNewOASRefreshJob(t *testing.T) {
	assert.Nil(t, NewOASRefreshJob(nil, context.Background()))

	stub := &refreshStub{result: models.OASRefreshResult{UpdatedCount: 2}, called: make(chan context.Context, 1)}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := NewOASRefreshJob(stub, parent)
	require.NotNil(t, job)
	defer job.Stop()

	select {
	case runCtx := <-stub.called:
		_, hasDeadline := runCtx.Deadline()
		assert.True(t, hasDeadline)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for startup refresh")
	}
}

func TestOASRefreshJobRunOnceHandlesErrorsAndCancellation(t *testing.T) {
	errorJob := &OASRefreshJob{
		refresher: &refreshStub{err: errors.New("boom")},
		ctx:       context.Background(),
	}
	errorJob.runOnce()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelStub := &refreshStub{}
	cancelJob := &OASRefreshJob{
		refresher: cancelStub,
		ctx:       ctx,
	}
	cancelJob.runOnce()
	assert.True(t, cancelStub.cancelSeen)
}

func TestOASRefreshJobErrorLogIncludesCountsAndHeap(t *testing.T) {
	var logBuffer bytes.Buffer
	logger, err := commonlogging.NewJSONLogger(&logBuffer, "api-register", "debug")
	require.NoError(t, err)
	previousLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	job := &OASRefreshJob{
		refresher: &refreshStub{
			result: models.OASRefreshResult{
				CandidateCount:   30,
				ProcessedCount:   25,
				UpdatedCount:     2,
				UnavailableCount: 1,
				FailedCount:      1,
			},
			err: errors.New("boom"),
		},
		ctx: context.Background(),
	}
	job.runOnce()

	var failed map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuffer.String()), "\n") {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["msg"] == "OAS refresh failed" {
			failed = record
		}
	}
	require.NotNil(t, failed)
	assert.Equal(t, "api-register", failed["app"])
	assert.Equal(t, float64(30), failed["candidate_count"])
	assert.Equal(t, float64(25), failed["processed_count"])
	assert.Equal(t, float64(1), failed["failed_count"])
	assert.Greater(t, failed["heap_alloc_bytes"].(float64), float64(0))
}

func TestOASRefreshJobStop(t *testing.T) {
	var nilJob *OASRefreshJob
	nilJob.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	job := &OASRefreshJob{ctx: ctx, cancel: cancel}

	job.Stop()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected stop to cancel context")
	}
}
