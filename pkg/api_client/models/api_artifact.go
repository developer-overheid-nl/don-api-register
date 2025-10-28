package models

import "time"

// ApiArtifact stores generated artifacts
// associated with an API. Content is stored as a blob.
type ApiArtifact struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	ApiID       string    `gorm:"column:api_id;index" json:"apiId"`
	Kind        string    `gorm:"column:kind;index" json:"kind"`
	Version     string    `gorm:"column:version;index" json:"version,omitempty"`
	Format      string    `gorm:"column:format;index" json:"format,omitempty"`
	Source      string    `gorm:"column:source;index" json:"source,omitempty"` // bv. original / derived
	Filename    string    `gorm:"column:filename" json:"filename"`
	ContentType string    `gorm:"column:content_type" json:"contentType"`
	Data        []byte    `gorm:"column:data;type:bytea" json:"-"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
}
