package entities

import "time"

// DatabaseCleanupSettings 保存存储维护配置；固定使用单行记录避免多版本配置并存。
type DatabaseCleanupSettings struct {
	ID                      int64     `gorm:"primaryKey"`
	RecordRequestDetails    bool      `gorm:"column:record_request_details;not null;default:true"`
	CleanupRequestLogs      bool      `gorm:"column:cleanup_request_logs;not null;default:true"`
	CleanupUsageLogs        bool      `gorm:"column:cleanup_usage_logs;not null;default:false"`
	RequestLogRetentionDays int       `gorm:"column:request_log_retention_days;not null;default:0"`
	UsageLogRetentionDays   int       `gorm:"column:usage_log_retention_days;not null;default:0"`
	MaxDatabaseSizeMB       int       `gorm:"column:max_database_size_mb;not null;default:0"`
	BackupRequestLogs       bool      `gorm:"column:backup_request_logs;not null;default:false"`
	BackupUsageLogs         bool      `gorm:"column:backup_usage_logs;not null;default:true"`
	BackupUsageIdentities   bool      `gorm:"column:backup_usage_identities;not null;default:true"`
	BackupAPIKeys           bool      `gorm:"column:backup_api_keys;not null;default:true"`
	BackupRedisInbox        bool      `gorm:"column:backup_redis_inbox;not null;default:false"`
	BackupModelPrices       bool      `gorm:"column:backup_model_prices;not null;default:true"`
	BackupHour              int       `gorm:"column:backup_hour;not null;default:4"`
	BackupMinute            int       `gorm:"column:backup_minute;not null;default:0"`
	MaxBackupCount          int       `gorm:"column:max_backup_count;not null;default:1"`
	CreatedAt               time.Time `gorm:"serializer:storageTime;not null"`
	UpdatedAt               time.Time `gorm:"serializer:storageTime;not null"`
}
