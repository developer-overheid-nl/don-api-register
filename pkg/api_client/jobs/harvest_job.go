package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commonlogging "github.com/developer-overheid-nl/don-register-common/logging"
	"github.com/robfig/cron/v3"
)

type Harvester interface {
	RunOnce(ctx context.Context, src models.HarvestSource) error
}

// ScheduleHarvest zet een cron job op die de opgegeven bronnen harvest
func ScheduleHarvest(ctx context.Context, svc Harvester, sources []models.HarvestSource) *cron.Cron {
	if ctx == nil {
		ctx = context.Background()
	}

	spec := "0 6 * * *"
	cronLogger := commonlogging.NewCronLogger(slog.Default(), "harvest")
	c := cron.New(cron.WithChain(
		cron.Recover(cronLogger),
		cron.SkipIfStillRunning(cronLogger),
	))

	// cron job
	_, err := c.AddFunc(spec, func() {
		jobCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		for _, src := range sources {
			if err := svc.RunOnce(jobCtx, src); err != nil {
				slog.ErrorContext(
					jobCtx,
					"scheduled harvest failed",
					"component", "harvest",
					"operation", "run",
					"source", src.Name,
					"error", err,
				)
			}
		}
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to schedule harvest",
			"component", "harvest",
			"operation", "schedule",
			"error", err,
		)
		return c
	}

	// start cron
	c.Start()

	// directe run bij opstart
	go func() {
		jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		for _, src := range sources {
			if err := svc.RunOnce(jobCtx, src); err != nil {
				slog.ErrorContext(
					jobCtx,
					"initial harvest failed",
					"component", "harvest",
					"operation", "run_initial",
					"source", src.Name,
					"error", err,
				)
			}
		}
	}()

	// stoppen als context sluit
	go func() {
		<-ctx.Done()
		c.Stop()
	}()
	return c
}

// SchedulePDOKHarvest bouwt een standaard PDOK-bron en plant de harvest
func SchedulePDOKHarvest(ctx context.Context, svc Harvester) *cron.Cron {
	src := models.HarvestSource{
		Name:            "pdok",
		IndexURL:        "https://api.pdok.nl/index.json",
		OrganisationUri: "https://www.pdok.nl",
		Contact: models.Contact{
			Name:  "PDOK Support",
			URL:   "https://www.pdok.nl/support1",
			Email: "support@pdok.nl",
		},
		UISuffix: "ui/",
		OASPath:  "openapi.json",
	}
	return ScheduleHarvest(ctx, svc, []models.HarvestSource{src})
}
