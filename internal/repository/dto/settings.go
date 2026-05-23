package dto

// DatabaseCleanupSettingsInput 是存储页提交的清理和备份配置；0 表示关闭对应阈值。
type DatabaseCleanupSettingsInput struct {
	RecordRequestDetails    bool
	CleanupRequestLogs      bool
	CleanupUsageLogs        bool
	RequestLogRetentionDays int
	UsageLogRetentionDays   int
	MaxDatabaseSizeMB       int
	BackupRequestLogs       bool
	BackupUsageLogs         bool
	BackupUsageIdentities   bool
	BackupAPIKeys           bool
	BackupRedisInbox        bool
	BackupModelPrices       bool
	BackupHour              int
	BackupMinute            int
	MaxBackupCount          int
}

// UsageRequestDetailCleanupResult 记录请求详情日志缓存清理结果。
type UsageRequestDetailCleanupResult struct {
	RetentionDeleted int64
	SizeDeleted      int64
}

// UsageEventCleanupResult 记录用量日志清理结果。
type UsageEventCleanupResult struct {
	RetentionDeleted int64
	SizeDeleted      int64
}
