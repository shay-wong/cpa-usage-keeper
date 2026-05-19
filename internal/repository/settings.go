package repository

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// databaseCleanupSettingsSingletonID 固定数据库清理配置只有一行，避免 UI 保存产生多份配置。
	databaseCleanupSettingsSingletonID int64 = 1
	// usageRequestDetailSizeCleanupBatchMax 限制单轮按库大小清理的最大删除批次，避免一次构造过大的 SQLite IN 条件。
	usageRequestDetailSizeCleanupBatchMax = 500
	// usageRequestDetailSizeCleanupBatchDivisor 控制按库大小清理时每轮最多删除约 10% 详情缓存。
	usageRequestDetailSizeCleanupBatchDivisor = 10
)

func GetDatabaseCleanupSettings(db *gorm.DB) (entities.DatabaseCleanupSettings, error) {
	if db == nil {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("database is nil")
	}
	var settings entities.DatabaseCleanupSettings
	if err := db.First(&settings, databaseCleanupSettingsSingletonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultDatabaseCleanupSettings(), nil
		}
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("get database cleanup settings: %w", err)
	}
	return settings, nil
}

func UpsertDatabaseCleanupSettings(db *gorm.DB, input dto.DatabaseCleanupSettingsInput) (entities.DatabaseCleanupSettings, error) {
	if db == nil {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("database is nil")
	}
	settings := entities.DatabaseCleanupSettings{
		ID:                      databaseCleanupSettingsSingletonID,
		RequestLogRetentionDays: input.RequestLogRetentionDays,
		MaxDatabaseSizeMB:       input.MaxDatabaseSizeMB,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"request_log_retention_days",
			"max_database_size_mb",
			"updated_at",
		}),
	}).Create(&settings).Error; err != nil {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("upsert database cleanup settings: %w", err)
	}
	return GetDatabaseCleanupSettings(db)
}

func CleanupUsageRequestDetails(db *gorm.DB, settings dto.DatabaseCleanupSettingsInput, now time.Time, sqlitePath string) (dto.UsageRequestDetailCleanupResult, error) {
	if db == nil {
		return dto.UsageRequestDetailCleanupResult{}, fmt.Errorf("database is nil")
	}
	var result dto.UsageRequestDetailCleanupResult
	if settings.RequestLogRetentionDays > 0 {
		deleted, err := cleanupUsageRequestDetailsByRetention(db, settings.RequestLogRetentionDays, now)
		if err != nil {
			return result, err
		}
		result.RetentionDeleted = deleted
	}
	if settings.MaxDatabaseSizeMB > 0 && strings.TrimSpace(sqlitePath) != "" {
		deleted, err := cleanupUsageRequestDetailsByDatabaseSize(db, settings.MaxDatabaseSizeMB, sqlitePath)
		if err != nil {
			return result, err
		}
		result.SizeDeleted = deleted
	}
	return result, nil
}

func defaultDatabaseCleanupSettings() entities.DatabaseCleanupSettings {
	return entities.DatabaseCleanupSettings{ID: databaseCleanupSettingsSingletonID}
}

func cleanupUsageRequestDetailsByRetention(db *gorm.DB, retentionDays int, now time.Time) (int64, error) {
	cutoff := timeutil.NormalizeStorageTime(now).AddDate(0, 0, -retentionDays)
	result := db.Where("fetched_at < ?", timeutil.FormatStorageTime(cutoff)).Delete(&entities.UsageRequestDetail{})
	if result.Error != nil {
		return 0, fmt.Errorf("cleanup usage request details by retention: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func cleanupUsageRequestDetailsByDatabaseSize(db *gorm.DB, maxDatabaseSizeMB int, sqlitePath string) (int64, error) {
	maxBytes := int64(maxDatabaseSizeMB) * 1024 * 1024
	var deleted int64
	for {
		sizeBytes, ok, err := sqliteDatabaseSizeBytes(sqlitePath)
		if err != nil || !ok {
			return deleted, err
		}
		if sizeBytes <= maxBytes {
			return deleted, nil
		}
		batchSize, err := usageRequestDetailCleanupBatchSize(db)
		if err != nil || batchSize == 0 {
			return deleted, err
		}
		batchDeleted, err := deleteOldestUsageRequestDetails(db, batchSize)
		if err != nil || batchDeleted == 0 {
			return deleted, err
		}
		deleted += batchDeleted
		if err := Vacuum(db); err != nil {
			return deleted, err
		}
	}
}

// GetSQLiteDatabaseSizeBytes 返回当前 SQLite 文件大小；内存库或无路径时 ok=false。
func GetSQLiteDatabaseSizeBytes(sqlitePath string) (int64, bool, error) {
	return sqliteDatabaseSizeBytes(sqlitePath)
}

func usageRequestDetailCleanupBatchSize(db *gorm.DB) (int, error) {
	var total int64
	if err := db.Model(&entities.UsageRequestDetail{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count usage request details for size cleanup: %w", err)
	}
	if total == 0 {
		return 0, nil
	}
	batchSize := int(total / usageRequestDetailSizeCleanupBatchDivisor)
	if batchSize < 1 {
		return 1, nil
	}
	return min(batchSize, usageRequestDetailSizeCleanupBatchMax), nil
}

func deleteOldestUsageRequestDetails(db *gorm.DB, batchSize int) (int64, error) {
	var ids []int64
	if err := db.Model(&entities.UsageRequestDetail{}).Order("fetched_at ASC, id ASC").Limit(batchSize).Pluck("id", &ids).Error; err != nil {
		return 0, fmt.Errorf("select old usage request details for size cleanup: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.Where("id IN ?", ids).Delete(&entities.UsageRequestDetail{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete old usage request details for size cleanup: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func sqliteDatabaseSizeBytes(sqlitePath string) (int64, bool, error) {
	trimmed := strings.TrimSpace(sqlitePath)
	if before, _, ok := strings.Cut(trimmed, "?"); ok {
		trimmed = before
	}
	if trimmed == "" || trimmed == ":memory:" {
		return 0, false, nil
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return 0, false, fmt.Errorf("check sqlite database size: %w", err)
	}
	return info.Size(), true, nil
}
