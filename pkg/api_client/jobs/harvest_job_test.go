package jobs_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/jobs"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
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

func (s *harvesterStub) RunOnce(ctx context.Context, src models.HarvestSource) error {
	_, hasDeadline := ctx.Deadline()
	call := harvestCall{src: src, hasDeadline: hasDeadline}

	s.mu.Lock()
	s.calls = append(s.calls, call)
	s.mu.Unlock()

	if s.callCh != nil {
		s.callCh <- call
	}

	if err, ok := s.errs[src.Name]; ok {
		return err
	}
	return nil
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
