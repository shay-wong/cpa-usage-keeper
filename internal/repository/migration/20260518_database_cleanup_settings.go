package migration

import (
	"fmt"

	"cpa-usage-keeper/internal/entities"
	"gorm.io/gorm"
)

// createDatabaseCleanupSettingsMigration 创建数据库自动清理设置表。
func createDatabaseCleanupSettingsMigration(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&entities.DatabaseCleanupSettings{}); err != nil {
		return fmt.Errorf("create database_cleanup_settings table: %w", err)
	}
	return nil
}
