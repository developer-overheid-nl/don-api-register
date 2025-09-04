package openapi

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

var lintLineRE = regexp.MustCompile(`^(\d+):(\d+)\s+(\w+)\s+(\S+)\s+(.*?)\s{2,}(.*)$`)

// ParseOutput converts the text output of Spectral into structured messages.
func ParseOutput(out string, timenow time.Time) []models.LintMessage {
	groups := make(map[string]*models.LintMessage)

	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		m := lintLineRE.FindStringSubmatch(line)
		if len(m) != 7 {
			continue
		}
		ln, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		severity := m[3]
		code := m[4]
		message := m[5]
		path := m[6]

		grp, exists := groups[code]
		if !exists {
			grp = &models.LintMessage{
				ID:        uuid.New().String(),
				Code:      code,
				Severity:  severity,
				Infos:     []models.LintMessageInfo{},
				CreatedAt: timenow,
				Line:      ln,
				Column:    col,
			}
			groups[code] = grp
		}
		grp.Infos = append(grp.Infos, models.LintMessageInfo{
			ID:            uuid.New().String(),
			LintMessageID: grp.ID,
			Message:       message,
			Path:          path,
		})
	}

	outGroups := make([]models.LintMessage, 0, len(groups))
	for _, grp := range groups {
		outGroups = append(outGroups, *grp)
	}
	return outGroups
}
