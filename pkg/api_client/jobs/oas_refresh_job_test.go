package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

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
	count      int
	err        error
	called     chan context.Context
	cancelSeen bool
}

func (s *refreshStub) RefreshChangedApis(ctx context.Context) (int, error) {
	if s.called != nil {
		s.called <- ctx
	}
	if ctx.Err() != nil {
		s.cancelSeen = true
	}
	return s.count, s.err
}

func TestNewOASRefreshJob(t *testing.T) {
	assert.Nil(t, NewOASRefreshJob(nil, context.Background()))

	stub := &refreshStub{count: 2, called: make(chan context.Context, 1)}
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
