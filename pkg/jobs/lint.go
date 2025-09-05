package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/robfig/cron/v3"
)

// ScheduleDailyLint sets up a cron job that lints all APIs every day.
func ScheduleDailyLint(ctx context.Context, svc *services.APIsAPIService) *cron.Cron {
	// voorkom overlap + vang panics
	c := cron.New(cron.WithChain(
		cron.Recover(cron.DefaultLogger),
		cron.SkipIfStillRunning(cron.DefaultLogger),
	))

	_, err := c.AddFunc("@every 15m", func() {
		fmt.Println("Starting linter job")

		jobCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := svc.LintAllApis(jobCtx); err != nil {
			fmt.Printf("lint job failed: %v\n", err)
		}
	})
	if err != nil {
		// je kunt hier ook return nil doen als je hard wilt falen
		fmt.Printf("failed to schedule lint job: %v\n", err)
	}

	c.Start()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return c
}
