package dto

// UpdateDatabaseCleanupSettingsInput 是存储页提交的清理、备份和请求详情记录配置；0 表示关闭对应阈值。
type UpdateDatabaseCleanupSettingsInput struct {
	RecordRequestDetails    bool `json:"record_request_details"`
	CleanupRequestLogs      bool `json:"cleanup_request_logs"`
	CleanupUsageLogs        bool `json:"cleanup_usage_logs"`
	RequestLogRetentionDays int  `json:"request_log_retention_days"`
	UsageLogRetentionDays   int  `json:"usage_log_retention_days"`
	MaxDatabaseSizeMB       int  `json:"max_database_size_mb"`
	BackupRequestLogs       bool `json:"backup_request_logs"`
	BackupUsageLogs         bool `json:"backup_usage_logs"`
	BackupUsageIdentities   bool `json:"backup_usage_identities"`
	BackupAPIKeys           bool `json:"backup_api_keys"`
	BackupRedisInbox        bool `json:"backup_redis_inbox"`
	BackupModelPrices       bool `json:"backup_model_prices"`
	BackupHour              int  `json:"backup_hour"`
	BackupMinute            int  `json:"backup_minute"`
	MaxBackupCount          int  `json:"max_backup_count"`
}

type StorageDomainInfo struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	TableNames  []string `json:"table_names"`
	Rows        int64    `json:"rows"`
	SizeBytes   int64    `json:"size_bytes"`
}

type BackupFileInfo struct {
	ID        string `json:"id"`
	FileName  string `json:"file_name"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

type StorageInfo struct {
	Settings                 UpdateDatabaseCleanupSettingsInput `json:"settings"`
	CurrentDatabaseSizeBytes *int64                             `json:"current_database_size_bytes,omitempty"`
	BackupTotalSizeBytes     int64                              `json:"backup_total_size_bytes"`
	BackupCount              int                                `json:"backup_count"`
	Domains                  []StorageDomainInfo                `json:"domains"`
	Backups                  []BackupFileInfo                   `json:"backups"`
}

type CreateBackupInput struct {
	RequestLogs     bool
	UsageLogs       bool
	UsageIdentities bool
	APIKeys         bool
	RedisInbox      bool
	ModelPrices     bool
}

type RestoreBackupInput struct {
	ID               string
	RequestLogs      bool
	UsageLogs        bool
	UsageIdentities  bool
	APIKeys          bool
	RedisInbox       bool
	ModelPrices      bool
	SkipSafetyBackup bool
}

type BackupOperationResult struct {
	Backup BackupFileInfo `json:"backup"`
}

type RestoreOperationResult struct {
	RestoredRequestLogs     bool `json:"restored_request_logs"`
	RestoredUsageLogs       bool `json:"restored_usage_logs"`
	RestoredUsageIdentities bool `json:"restored_usage_identities"`
	RestoredAPIKeys         bool `json:"restored_api_keys"`
	RestoredRedisInbox      bool `json:"restored_redis_inbox"`
	RestoredModelPrices     bool `json:"restored_model_prices"`
}
