package migration

import (
	"fmt"

	"cpa-usage-keeper/internal/entities"

	"gorm.io/gorm"
)

// createUsageRequestDetailsMigration 创建 request log 详情缓存表和 request_id 唯一索引。
func createUsageRequestDetailsMigration(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&entities.UsageRequestDetail{}); err != nil {
		return fmt.Errorf("create usage_request_details table: %w", err)
	}
	return nil
}
