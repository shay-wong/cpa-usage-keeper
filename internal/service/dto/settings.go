package dto

// UpdateDatabaseCleanupSettingsInput 是设置页提交的数据库自动清理配置；0 表示关闭对应规则。
type UpdateDatabaseCleanupSettingsInput struct {
	RequestLogRetentionDays int
	MaxDatabaseSizeMB       int
}
