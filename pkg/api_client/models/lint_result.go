package models

import (
	"time"
)

// LintResult stores the output of a linter run for an API
// so we can keep a history of lint outcomes.
type LintResult struct {
	ID        string        `gorm:"column:id;primaryKey"`
	ApiID     string        `gorm:"column:api_id"`
	Result    string        `gorm:"column:result"`
	CreatedAt time.Time     `gorm:"column:created_at"`
	Messages  []LintMessage `gorm:"foreignKey:LintResultID" json:"messages,omitempty"`
}

type LintMessage struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	LintResultID string `gorm:"column:lint_result_id" json:"lintResultId"`
	Line         int    `gorm:"column:line" json:"line"`
	Column       int    `gorm:"column:column" json:"column"`
	Severity     string `gorm:"column:severity" json:"severity"`
	Code         string `gorm:"column:code" json:"code"`
	Message      string `gorm:"column:message" json:"message"`
	Path         string `gorm:"column:path" json:"path"`
}
