package services

import (
	"context"
	"errors"
	"log"
	"time"
)

const (
	refreshHour   = 7
	refreshMinute = 0
	runTimeout    = 30 * time.Minute
	refreshPeriod = 24 * time.Hour
)

// OASRefreshService draait dagelijks een refresh-run om 07:00.
type OASRefreshService struct {
	apiService *APIsAPIService
	location   *time.Location
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewOASRefreshService start direct een dagelijkse job. Parent context kan nil zijn.
func NewOASRefreshService(apiService *APIsAPIService, parentCtx context.Context) *OASRefreshService {
	if apiService == nil {
		return nil
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	svc := &OASRefreshService{
		apiService: apiService,
		location:   time.Local,
		ctx:        ctx,
		cancel:     cancel,
	}
	go svc.loop()
	return svc
}

// Stop beëindigt de job.
func (s *OASRefreshService) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
}

func (s *OASRefreshService) loop() {
	for {
		delay := time.Until(nextRunAt(time.Now().In(s.location), refreshHour, refreshMinute))
		timer := time.NewTimer(delay)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.runOnce()
		}
	}
}

func (s *OASRefreshService) runOnce() {
	runCtx, cancel := context.WithTimeout(s.ctx, runTimeout)
	defer cancel()

	count, err := s.apiService.RefreshChangedApis(runCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Printf("[oas-refresh] run afgebroken: %v", err)
		} else {
			log.Printf("[oas-refresh] run mislukt: %v", err)
		}
		return
	}
	log.Printf("[oas-refresh] run gereed; %d APIs bijgewerkt", count)
}

func nextRunAt(now time.Time, hour, minute int) time.Time {
	loc := now.Location()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(now) {
		candidate = candidate.Add(refreshPeriod)
	}
	return candidate
}
