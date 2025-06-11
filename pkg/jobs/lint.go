package jobs

import (
	"context"
	"github.com/developer-overheid-nl/don-api-register/pkg/tools"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/robfig/cron/v3"
)

// ScheduleDailyLint sets up a cron job that lints all APIs every day.
func ScheduleDailyLint(ctx context.Context, svc *services.APIsAPIService) *cron.Cron {
	c := cron.New()
	_, _ = c.AddFunc("@daily", func() {
		tools.Dispatch(context.Background(), "lint_all", func(ctx context.Context) error {
			return svc.LintAllApis(ctx)
		})
	})
	c.Start()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()
	return c
}
