package migration

import (
	"fmt"

	"cpa-usage-keeper/internal/entities"
	"gorm.io/gorm"
)

// extendDatabaseCleanupSettingsMigration 补齐存储页新增的清理、备份和请求详情记录配置列。
func extendDatabaseCleanupSettingsMigration(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&entities.DatabaseCleanupSettings{}); err != nil {
		return fmt.Errorf("extend database_cleanup_settings table: %w", err)
	}
	return nil
}
