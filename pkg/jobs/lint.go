package jobs

import (
	"context"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/robfig/cron/v3"
)

// ScheduleDailyLint sets up a cron job that lints all APIs every day.
func ScheduleDailyLint(ctx context.Context, svc *services.APIsAPIService) *cron.Cron {
	c := cron.New()
	_, _ = c.AddFunc("@daily", func() {
		_ = svc.LintAllApis(context.Background())
	})
	c.Start()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()
	return c
}
