package dto

// DatabaseCleanupSettingsInput 是数据库自动清理配置写入参数；0 表示关闭对应清理规则。
type DatabaseCleanupSettingsInput struct {
	RequestLogRetentionDays int
	MaxDatabaseSizeMB       int
}

// UsageRequestDetailCleanupResult 记录请求详情日志缓存清理结果。
type UsageRequestDetailCleanupResult struct {
	RetentionDeleted int64
	SizeDeleted      int64
}
