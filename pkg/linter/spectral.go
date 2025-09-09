package linter

import (
    "context"
    "errors"
    "fmt"
    "log"
    "os"
    "os/exec"
    "time"
)

// LintURL runs the spectral linter for a single URL.
func LintURL(ctx context.Context, url string) (string, error) {
    const ruleset = "https://static.developer.overheid.nl/adr/2.1/ruleset.yaml"

    // Basic diagnostics to help on servers
    if path, err := exec.LookPath("spectral"); err != nil {
        log.Printf("[lint:spectral] spectral binary not found in PATH (%s): %v", os.Getenv("PATH"), err)
    } else {
        log.Printf("[lint:spectral] using spectral at: %s", path)
    }
    if wd, err := os.Getwd(); err == nil {
        log.Printf("[lint:spectral] working dir: %s", wd)
    }
    log.Printf("[lint:spectral] command: spectral lint -F error -D -r %s %s", ruleset, url)

    cmd := exec.CommandContext(
        ctx,
        "spectral", "lint",
        "-F", "error",
        "-D",
        "-r", ruleset,
        url,
    )

    started := time.Now()
    output, err := cmd.CombinedOutput()
    dur := time.Since(started)
    log.Printf("[lint:spectral] finished in %s (bytes=%d)", dur, len(output))

    if err != nil {
        var ee *exec.ExitError
        switch {
        case errors.As(err, &ee):
            // Non-zero exit means lint errors were found; not a fatal error for us
            log.Printf("[lint:spectral] non-zero exit (code=%d); treating as lint findings", ee.ExitCode())
            return string(output), nil
        default:
            log.Printf("[lint:spectral] execution error: %v", err)
            return string(output), fmt.Errorf("spectral lint failed for %s: %w", url, err)
        }
    }
    return string(output), nil
}
