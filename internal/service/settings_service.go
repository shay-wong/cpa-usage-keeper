package service

import (
	"context"
	"fmt"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository"
	repodto "cpa-usage-keeper/internal/repository/dto"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"gorm.io/gorm"
)

type DatabaseCleanupSettingsSnapshot struct {
	Settings                 entities.DatabaseCleanupSettings
	CurrentDatabaseSizeBytes *int64
}

type DatabaseSettingsProvider interface {
	GetDatabaseCleanupSettings(context.Context) (DatabaseCleanupSettingsSnapshot, error)
	UpdateDatabaseCleanupSettings(context.Context, servicedto.UpdateDatabaseCleanupSettingsInput) (DatabaseCleanupSettingsSnapshot, error)
}

type databaseSettingsService struct {
	db         *gorm.DB
	sqlitePath string
}

func NewDatabaseSettingsService(db *gorm.DB, sqlitePath string) DatabaseSettingsProvider {
	return &databaseSettingsService{db: db, sqlitePath: sqlitePath}
}

func (s *databaseSettingsService) GetDatabaseCleanupSettings(context.Context) (DatabaseCleanupSettingsSnapshot, error) {
	settings, err := repository.GetDatabaseCleanupSettings(s.db)
	if err != nil {
		return DatabaseCleanupSettingsSnapshot{}, err
	}
	snapshot := DatabaseCleanupSettingsSnapshot{Settings: settings}
	sizeBytes, ok, err := repository.GetSQLiteDatabaseSizeBytes(s.sqlitePath)
	if err != nil {
		return DatabaseCleanupSettingsSnapshot{}, err
	}
	if ok {
		snapshot.CurrentDatabaseSizeBytes = &sizeBytes
	}
	return snapshot, nil
}

func (s *databaseSettingsService) UpdateDatabaseCleanupSettings(ctx context.Context, input servicedto.UpdateDatabaseCleanupSettingsInput) (DatabaseCleanupSettingsSnapshot, error) {
	if input.RequestLogRetentionDays < 0 {
		return DatabaseCleanupSettingsSnapshot{}, fmt.Errorf("request log retention days must be non-negative")
	}
	if input.MaxDatabaseSizeMB < 0 {
		return DatabaseCleanupSettingsSnapshot{}, fmt.Errorf("max database size MB must be non-negative")
	}
	if _, err := repository.UpsertDatabaseCleanupSettings(s.db, repodto.DatabaseCleanupSettingsInput{
		RequestLogRetentionDays: input.RequestLogRetentionDays,
		MaxDatabaseSizeMB:       input.MaxDatabaseSizeMB,
	}); err != nil {
		return DatabaseCleanupSettingsSnapshot{}, err
	}
	return s.GetDatabaseCleanupSettings(ctx)
}
