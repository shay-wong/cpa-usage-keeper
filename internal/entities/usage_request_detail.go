package entities

import "time"

// UsageRequestDetail 缓存 CLIProxyAPI request log 详情，按 request_id 去重保存。
type UsageRequestDetail struct {
	ID        int64     `gorm:"primaryKey"`
	RequestID string    `gorm:"column:request_id;not null;uniqueIndex:uniq_usage_request_details_request_id"`
	Content   string    `gorm:"column:content;not null"`
	Source    string    `gorm:"column:source;not null"`
	FetchedAt time.Time `gorm:"serializer:storageTime;not null;column:fetched_at"`
	CreatedAt time.Time `gorm:"serializer:storageTime;not null;column:created_at"`
	UpdatedAt time.Time `gorm:"serializer:storageTime;not null;column:updated_at"`
}
