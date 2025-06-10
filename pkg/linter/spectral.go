package linter

import (
	"context"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"os/exec"

	"golang.org/x/sync/errgroup"
)

// LintURL runs the spectral linter for a single URL.
func LintURL(ctx context.Context, url string) (string, error) {
	cmd := exec.CommandContext(ctx, "spectral", "lint", "-r", "https://static.developer.overheid.nl/adr/ruleset.yaml", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("spectral lint failed for %s: %w", url, err)
	}
	return string(output), nil
}

func LintURLStructured(ctx context.Context, url string) ([]models.LintMessage, error) {
	out, err := LintURL(ctx, url)
	msgs := ParseOutput(out)
	return msgs, err
}

func LintURLsStructured(ctx context.Context, urls []string, concurrency int) (map[string][]models.LintMessage, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	g, ctx := errgroup.WithContext(ctx)
	results := make(map[string][]models.LintMessage)

	for _, u := range urls {
		u := u
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			msgs, err := LintURLStructured(ctx, u)
			results[u] = msgs
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}
	return results, nil
}
