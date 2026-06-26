package models

import "time"

const (
	ProcessingToolOASBundle      = "oas_bundle"
	ProcessingToolOASFetch       = "oas_fetch"
	ProcessingToolOASArtifacts   = "oas_artifacts"
	ProcessingToolLint           = "lint"
	ProcessingToolPostman        = "postman"
	ProcessingToolArazzoMarkdown = "arazzo_markdown"
	ProcessingToolArazzoMermaid  = "arazzo_mermaid"
	ProcessingToolTypesense      = "typesense"

	ProcessingStatusFailed            = "failed"
	ProcessingStatusFallbackSucceeded = "fallback_succeeded"
)

type ApiProcessingEvent struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	ApiID     string    `gorm:"column:api_id;index" json:"apiId"`
	Tool      string    `gorm:"column:tool;index" json:"tool"`
	Status    string    `gorm:"column:status;index" json:"status"`
	Message   string    `gorm:"column:message" json:"message"`
	Detail    string    `gorm:"column:detail;type:text" json:"detail,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;index" json:"createdAt"`
}
