package entities

import "time"

// DatabaseCleanupSettings 保存数据库自动清理配置；固定使用单行记录避免多版本配置并存。
type DatabaseCleanupSettings struct {
	ID                      int64     `gorm:"primaryKey"`
	RequestLogRetentionDays int       `gorm:"column:request_log_retention_days;not null;default:0"`
	MaxDatabaseSizeMB       int       `gorm:"column:max_database_size_mb;not null;default:0"`
	CreatedAt               time.Time `gorm:"serializer:storageTime;not null"`
	UpdatedAt               time.Time `gorm:"serializer:storageTime;not null"`
}
