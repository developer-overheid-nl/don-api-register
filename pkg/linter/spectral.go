package linter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func findSpectral() (string, error) {
	p, err := exec.LookPath("spectral")
	if err != nil {
		return "", fmt.Errorf("spectral niet gevonden op PATH (%s): %w", os.Getenv("PATH"), err)
	}
	return p, nil
}

// LintURL runs the spectral linter for a single URL.
func LintURL(ctx context.Context, url string) (string, error) {
	sp, err := findSpectral()
	if err != nil {
		return "", err
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		ctx = cctx
	}

	args := []string{
		"lint",
		"-f", "json",
		"-F", "error", // non-zero exit alleen bij errors in regels
		"-D",
		"-r", "https://static.developer.overheid.nl/adr/2.1/ruleset.yaml",
		url,
	}
	cmd := exec.CommandContext(ctx, sp, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	log.Printf("[lint:spectral] exec: %s %s", sp, strings.Join(args, " "))

	runErr := cmd.Run()
	dur := time.Since(start)
	outStr := stdout.String()
	errStr := stderr.String()

	log.Printf("[lint:spectral] finished in %s (bytes=%d)", dur, len(outStr))

	// Als er een exitstatus is en stdout is leeg: behandel als fout (bv. context kill, netwerk, OOM)
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			log.Printf("[lint:spectral] non-zero exit (code=%d)", ee.ExitCode())
			if strings.TrimSpace(outStr) == "" {
				if ctx.Err() != nil {
					return "", ctx.Err() // “context canceled/deadline exceeded”
				}
				if strings.TrimSpace(errStr) != "" {
					return "", fmt.Errorf("spectral failed: %s", strings.TrimSpace(errStr))
				}
				return "", fmt.Errorf("spectral failed with exit code %d", ee.ExitCode())
			}
			// stdout bevat JSON → geef het terug (lint findings)
			return outStr, nil
		}
		// Andere exec-fout
		return "", fmt.Errorf("spectral exec failed: %w", runErr)
	}

	return outStr, nil
}
