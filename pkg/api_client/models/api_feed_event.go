package models

import "time"

const (
	ApiFeedEventADRScoreChanged  = "adr_score_changed"
	ApiFeedEventLifecycleChanged = "lifecycle_changed"
	ApiFeedEventOASHashChanged   = "oas_hash_changed"
	ApiFeedEventOASUnavailable   = "oas_unavailable"
)

type ApiFeedEvent struct {
	ID          string    `gorm:"column:id;primaryKey"`
	ApiID       string    `gorm:"column:api_id;index"`
	Type        string    `gorm:"column:type;index"`
	Title       string    `gorm:"column:title"`
	Description string    `gorm:"column:description"`
	OldValue    string    `gorm:"column:old_value"`
	NewValue    string    `gorm:"column:new_value"`
	CreatedAt   time.Time `gorm:"column:created_at;index"`
}

type ApiFeedEventText struct {
	Title       string
	Description string
}
