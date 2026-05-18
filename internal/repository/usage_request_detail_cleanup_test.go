package repository

import (
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
	"gorm.io/gorm"
)

func TestCleanupUsageRequestDetailsByRetentionDeletesOnlyOldDetails(t *testing.T) {
	db := openRequestDetailCleanupTestDatabase(t)
	now := time.Date(2026, 5, 18, 3, 0, 0, 0, time.UTC)
	oldFetchedAt := now.AddDate(0, 0, -8)
	recentFetchedAt := now.AddDate(0, 0, -6)
	seedUsageRequestDetail(t, db, "old-detail", oldFetchedAt)
	seedUsageRequestDetail(t, db, "recent-detail", recentFetchedAt)
	if err := db.Create(&entities.UsageEvent{EventKey: "event-1", RequestID: "old-detail", Timestamp: oldFetchedAt}).Error; err != nil {
		t.Fatalf("seed usage event: %v", err)
	}

	result, err := CleanupUsageRequestDetails(db, dto.DatabaseCleanupSettingsInput{RequestLogRetentionDays: 7}, now, "")
	if err != nil {
		t.Fatalf("CleanupUsageRequestDetails returned error: %v", err)
	}
	if result.RetentionDeleted != 1 {
		t.Fatalf("expected one detail deleted by retention, got %+v", result)
	}

	var detailIDs []string
	if err := db.Model(&entities.UsageRequestDetail{}).Order("request_id asc").Pluck("request_id", &detailIDs).Error; err != nil {
		t.Fatalf("load remaining request details: %v", err)
	}
	if len(detailIDs) != 1 || detailIDs[0] != "recent-detail" {
		t.Fatalf("expected only recent detail to remain, got %#v", detailIDs)
	}
	var eventCount int64
	if err := db.Model(&entities.UsageEvent{}).Count(&eventCount).Error; err != nil {
		t.Fatalf("count usage events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected usage events untouched, got %d", eventCount)
	}
}

func TestCleanupUsageRequestDetailsDisabledWhenSettingsAreZero(t *testing.T) {
	db := openRequestDetailCleanupTestDatabase(t)
	now := time.Date(2026, 5, 18, 3, 0, 0, 0, time.UTC)
	seedUsageRequestDetail(t, db, "old-detail", now.AddDate(0, 0, -365))

	result, err := CleanupUsageRequestDetails(db, dto.DatabaseCleanupSettingsInput{}, now, "")
	if err != nil {
		t.Fatalf("CleanupUsageRequestDetails returned error: %v", err)
	}
	if result.RetentionDeleted != 0 || result.SizeDeleted != 0 {
		t.Fatalf("expected disabled cleanup to be no-op, got %+v", result)
	}
	var count int64
	if err := db.Model(&entities.UsageRequestDetail{}).Count(&count).Error; err != nil {
		t.Fatalf("count request details: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected request detail to remain, got %d", count)
	}
}

func seedUsageRequestDetail(t *testing.T, db *gorm.DB, requestID string, fetchedAt time.Time) {
	t.Helper()
	if err := db.Create(&entities.UsageRequestDetail{
		RequestID: requestID,
		Content:   "synthetic request detail",
		Source:    "test",
		FetchedAt: fetchedAt,
		CreatedAt: fetchedAt,
		UpdatedAt: fetchedAt,
	}).Error; err != nil {
		t.Fatalf("seed request detail %s: %v", requestID, err)
	}
}

func openRequestDetailCleanupTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "request-detail-cleanup.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	return db
}
