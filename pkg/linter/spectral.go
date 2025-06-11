package linter

import (
	"context"
	"fmt"
	"os/exec"
)

// LintURL runs the spectral linter for a single URL.
func LintURL(ctx context.Context, url string) (string, error) {
	cmd := exec.CommandContext(ctx, "spectral", "lint", "-r", "https://static.developer.overheid.nl/adr/2.1/ruleset.yaml", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("spectral lint failed for %s: %w", url, err)
	}
	return string(output), nil
}
