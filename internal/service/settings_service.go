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

type DatabaseSettingsProvider interface {
	GetDatabaseCleanupSettings(context.Context) (entities.DatabaseCleanupSettings, error)
	UpdateDatabaseCleanupSettings(context.Context, servicedto.UpdateDatabaseCleanupSettingsInput) (entities.DatabaseCleanupSettings, error)
}

type databaseSettingsService struct {
	db *gorm.DB
}

func NewDatabaseSettingsService(db *gorm.DB) DatabaseSettingsProvider {
	return &databaseSettingsService{db: db}
}

func (s *databaseSettingsService) GetDatabaseCleanupSettings(context.Context) (entities.DatabaseCleanupSettings, error) {
	return repository.GetDatabaseCleanupSettings(s.db)
}

func (s *databaseSettingsService) UpdateDatabaseCleanupSettings(_ context.Context, input servicedto.UpdateDatabaseCleanupSettingsInput) (entities.DatabaseCleanupSettings, error) {
	if input.RequestLogRetentionDays < 0 {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("request log retention days must be non-negative")
	}
	if input.MaxDatabaseSizeMB < 0 {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("max database size MB must be non-negative")
	}
	return repository.UpsertDatabaseCleanupSettings(s.db, repodto.DatabaseCleanupSettingsInput{
		RequestLogRetentionDays: input.RequestLogRetentionDays,
		MaxDatabaseSizeMB:       input.MaxDatabaseSizeMB,
	})
}
