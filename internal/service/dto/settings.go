package dto

// UpdateDatabaseCleanupSettingsInput 是存储页提交的清理、备份和请求详情记录配置；0 表示关闭对应阈值。
type UpdateDatabaseCleanupSettingsInput struct {
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

type StorageDomainInfo struct {
	Key         string
	Label       string
	Description string
	TableNames  []string
	Rows        int64
	SizeBytes   int64
}

type BackupFileInfo struct {
	ID        string
	FileName  string
	SizeBytes int64
	CreatedAt string
}

type StorageInfo struct {
	Settings                 UpdateDatabaseCleanupSettingsInput
	CurrentDatabaseSizeBytes *int64
	BackupTotalSizeBytes     int64
	BackupCount              int
	Domains                  []StorageDomainInfo
	Backups                  []BackupFileInfo
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
	Backup BackupFileInfo
}

type RestoreOperationResult struct {
	RestoredRequestLogs     bool
	RestoredUsageLogs       bool
	RestoredUsageIdentities bool
	RestoredAPIKeys         bool
	RestoredRedisInbox      bool
	RestoredModelPrices     bool
}
