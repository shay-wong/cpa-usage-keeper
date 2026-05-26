package migration

import (
	"fmt"

	"cpa-usage-keeper/internal/entities"
	"gorm.io/gorm"
)

// extendDatabaseCleanupBackupDomainsMigration 补齐存储页新增的备份数据域配置列。
func extendDatabaseCleanupBackupDomainsMigration(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&entities.DatabaseCleanupSettings{}); err != nil {
		return fmt.Errorf("extend database cleanup backup domains: %w", err)
	}
	return nil
}
