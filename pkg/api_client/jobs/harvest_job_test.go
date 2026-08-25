package jobs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/jobs"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commonlogging "github.com/developer-overheid-nl/don-register-common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type harvestCall struct {
	src         models.HarvestSource
	hasDeadline bool
}

type harvesterStub struct {
	mu     sync.Mutex
	calls  []harvestCall
	errs   map[string]error
	callCh chan harvestCall
}

func (s *harvesterStub) RunOnce(ctx context.Context, src models.HarvestSource) (models.HarvestResult, error) {
	_, hasDeadline := ctx.Deadline()
	call := harvestCall{src: src, hasDeadline: hasDeadline}

	s.mu.Lock()
	s.calls = append(s.calls, call)
	s.mu.Unlock()

	if s.callCh != nil {
		s.callCh <- call
	}

	if err, ok := s.errs[src.Name]; ok {
		return models.HarvestResult{CandidateCount: 1, FailedCount: 1}, err
	}
	return models.HarvestResult{CandidateCount: 1, SkippedCount: 1}, nil
}

func waitForHarvestCall(t *testing.T, ch <-chan harvestCall) harvestCall {
	t.Helper()

	select {
	case call := <-ch:
		return call
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for harvest call")
		return harvestCall{}
	}
}

func TestScheduleHarvest_RunsSourcesImmediatelyOnStartup(t *testing.T) {
	stub := &harvesterStub{callCh: make(chan harvestCall, 2)}
	sources := []models.HarvestSource{
		{Name: "source-a", IndexURL: "https://example.com/a/index.json"},
		{Name: "source-b", IndexURL: "https://example.com/b/index.json"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := jobs.ScheduleHarvest(ctx, stub, sources)
	require.NotNil(t, c)

	first := waitForHarvestCall(t, stub.callCh)
	second := waitForHarvestCall(t, stub.callCh)

	assert.Equal(t, "source-a", first.src.Name)
	assert.Equal(t, "source-b", second.src.Name)
	assert.True(t, first.hasDeadline)
	assert.True(t, second.hasDeadline)
}

func TestScheduleHarvest_CronEntryRunsHarvestAgain(t *testing.T) {
	stub := &harvesterStub{callCh: make(chan harvestCall, 2)}
	source := models.HarvestSource{Name: "source-a", IndexURL: "https://example.com/a/index.json"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := jobs.ScheduleHarvest(ctx, stub, []models.HarvestSource{source})
	require.NotNil(t, c)

	startupCall := waitForHarvestCall(t, stub.callCh)
	assert.Equal(t, source.Name, startupCall.src.Name)

	entries := c.Entries()
	require.Len(t, entries, 1)

	entries[0].Job.Run()
	scheduledCall := waitForHarvestCall(t, stub.callCh)

	assert.Equal(t, source.Name, scheduledCall.src.Name)
	assert.True(t, scheduledCall.hasDeadline)
}

func TestScheduleHarvest_ContinuesAfterRunOnceError(t *testing.T) {
	stub := &harvesterStub{
		callCh: make(chan harvestCall, 2),
		errs: map[string]error{
			"source-a": errors.New("boom"),
		},
	}
	sources := []models.HarvestSource{
		{Name: "source-a", IndexURL: "https://example.com/a/index.json"},
		{Name: "source-b", IndexURL: "https://example.com/b/index.json"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := jobs.ScheduleHarvest(ctx, stub, sources)
	require.NotNil(t, c)

	first := waitForHarvestCall(t, stub.callCh)
	second := waitForHarvestCall(t, stub.callCh)

	assert.Equal(t, "source-a", first.src.Name)
	assert.Equal(t, "source-b", second.src.Name)
}

func TestScheduleHarvestDoesNotDuplicateServiceErrorLog(t *testing.T) {
	stub := &harvesterStub{
		callCh: make(chan harvestCall, 1),
		errs:   map[string]error{"source-a": errors.New("already logged by service")},
	}
	var logBuffer bytes.Buffer
	logger, err := commonlogging.NewJSONLogger(&logBuffer, "api-register", "debug")
	require.NoError(t, err)
	previousLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobs.ScheduleHarvest(ctx, stub, []models.HarvestSource{{Name: "source-a", IndexURL: "https://example.com/index.json"}})
	waitForHarvestCall(t, stub.callCh)

	for _, line := range strings.Split(strings.TrimSpace(logBuffer.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		assert.NotEqual(t, "initial harvest failed", record["msg"])
		assert.NotEqual(t, "scheduled harvest failed", record["msg"])
	}
}

func TestSchedulePDOKHarvest_UsesExpectedDefaultSource(t *testing.T) {
	stub := &harvesterStub{callCh: make(chan harvestCall, 1)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := jobs.SchedulePDOKHarvest(ctx, stub)
	require.NotNil(t, c)

	call := waitForHarvestCall(t, stub.callCh)

	assert.Equal(t, "pdok", call.src.Name)
	assert.Equal(t, "https://api.pdok.nl/index.json", call.src.IndexURL)
	assert.Equal(t, "https://www.pdok.nl", call.src.OrganisationUri)
	assert.Equal(t, "ui/", call.src.UISuffix)
	assert.Equal(t, "openapi.json", call.src.OASPath)
	assert.Equal(t, "PDOK Support", call.src.Contact.Name)
	assert.Equal(t, "support@pdok.nl", call.src.Contact.Email)
	assert.Equal(t, "https://www.pdok.nl/support1", call.src.Contact.URL)
}
