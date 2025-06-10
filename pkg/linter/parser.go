package linter

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

var lintLineRE = regexp.MustCompile(`^(\d+):(\d+)\s+(\w+)\s+(\S+)\s+(.*?)\s{2,}(.*)$`)

// ParseOutput converts the text output of Spectral into structured messages.
func ParseOutput(out string) []models.LintMessage {
	var msgs []models.LintMessage
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := lintLineRE.FindStringSubmatch(line)
		if len(matches) != 7 {
			continue
		}
		ln, _ := strconv.Atoi(matches[1])
		col, _ := strconv.Atoi(matches[2])
		msgs = append(msgs, models.LintMessage{
			Line:     ln,
			Column:   col,
			Severity: matches[3],
			Code:     matches[4],
			Message:  matches[5],
			Path:     matches[6],
		})
	}
	return msgs
}
