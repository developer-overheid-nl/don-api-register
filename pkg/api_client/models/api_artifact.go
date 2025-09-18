package models

import "time"

// ApiArtifact stores generated artifacts (e.g., Bruno ZIP, Postman JSON)
// associated with an API. Content is stored as a blob.
type ApiArtifact struct {
    ID          string    `gorm:"column:id;primaryKey" json:"id"`
    ApiID       string    `gorm:"column:api_id;index" json:"apiId"`
    Kind        string    `gorm:"column:kind;index" json:"kind"` // bruno | postman
    Filename    string    `gorm:"column:filename" json:"filename"`
    ContentType string    `gorm:"column:content_type" json:"contentType"`
    Data        []byte    `gorm:"column:data;type:bytea" json:"-"`
    CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
}

