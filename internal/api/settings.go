package api

import (
	"errors"
	"net/http"

	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"github.com/gin-gonic/gin"
)

type databaseCleanupSettingsResponse struct {
	RecordRequestDetails     bool   `json:"record_request_details"`
	CleanupRequestLogs       bool   `json:"cleanup_request_logs"`
	CleanupUsageLogs         bool   `json:"cleanup_usage_logs"`
	RequestLogRetentionDays  int    `json:"request_log_retention_days"`
	UsageLogRetentionDays    int    `json:"usage_log_retention_days"`
	MaxDatabaseSizeMB        int    `json:"max_database_size_mb"`
	BackupRequestLogs        bool   `json:"backup_request_logs"`
	BackupUsageLogs          bool   `json:"backup_usage_logs"`
	BackupUsageIdentities    bool   `json:"backup_usage_identities"`
	BackupAPIKeys            bool   `json:"backup_api_keys"`
	BackupRedisInbox         bool   `json:"backup_redis_inbox"`
	BackupModelPrices        bool   `json:"backup_model_prices"`
	BackupHour               int    `json:"backup_hour"`
	BackupMinute             int    `json:"backup_minute"`
	MaxBackupCount           int    `json:"max_backup_count"`
	CurrentDatabaseSizeBytes *int64 `json:"current_database_size_bytes,omitempty"`
}

type updateDatabaseCleanupSettingsRequest struct {
	RecordRequestDetails    bool `json:"record_request_details"`
	CleanupRequestLogs      bool `json:"cleanup_request_logs"`
	CleanupUsageLogs        bool `json:"cleanup_usage_logs"`
	RequestLogRetentionDays int  `json:"request_log_retention_days"`
	UsageLogRetentionDays   int  `json:"usage_log_retention_days"`
	MaxDatabaseSizeMB       int  `json:"max_database_size_mb"`
	BackupRequestLogs       bool `json:"backup_request_logs"`
	BackupUsageLogs         bool `json:"backup_usage_logs"`
	BackupUsageIdentities   bool `json:"backup_usage_identities"`
	BackupAPIKeys           bool `json:"backup_api_keys"`
	BackupRedisInbox        bool `json:"backup_redis_inbox"`
	BackupModelPrices       bool `json:"backup_model_prices"`
	BackupHour              int  `json:"backup_hour"`
	BackupMinute            int  `json:"backup_minute"`
	MaxBackupCount          int  `json:"max_backup_count"`
}

type createBackupRequest struct {
	RequestLogs     bool `json:"request_logs"`
	UsageLogs       bool `json:"usage_logs"`
	UsageIdentities bool `json:"usage_identities"`
	APIKeys         bool `json:"api_keys"`
	RedisInbox      bool `json:"redis_inbox"`
	ModelPrices     bool `json:"model_prices"`
}

type restoreBackupRequest struct {
	ID               string `json:"id"`
	RequestLogs      bool   `json:"request_logs"`
	UsageLogs        bool   `json:"usage_logs"`
	UsageIdentities  bool   `json:"usage_identities"`
	APIKeys          bool   `json:"api_keys"`
	RedisInbox       bool   `json:"redis_inbox"`
	ModelPrices      bool   `json:"model_prices"`
	SkipSafetyBackup bool   `json:"skip_safety_backup"`
}

func registerSettingsRoutes(router gin.IRoutes, provider service.DatabaseSettingsProvider) {
	router.GET("/settings/database", func(c *gin.Context) {
		if provider == nil {
			c.JSON(http.StatusOK, databaseCleanupSettingsResponse{})
			return
		}
		snapshot, err := provider.GetDatabaseCleanupSettings(c.Request.Context())
		if err != nil {
			writeInternalError(c, "get database cleanup settings failed", err)
			return
		}
		c.JSON(http.StatusOK, mapDatabaseSettingsResponse(snapshot))
	})

	router.PUT("/settings/database", func(c *gin.Context) {
		if provider == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "database settings provider is not configured"})
			return
		}
		var request updateDatabaseCleanupSettingsRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		snapshot, err := provider.UpdateDatabaseCleanupSettings(c.Request.Context(), mapDatabaseSettingsInput(request))
		if err != nil {
			if errors.Is(err, service.ErrStorageSettingsInvalid) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			writeInternalError(c, "update database cleanup settings failed", err)
			return
		}
		c.JSON(http.StatusOK, mapDatabaseSettingsResponse(snapshot))
	})

	router.GET("/settings/storage", func(c *gin.Context) {
		if provider == nil {
			c.JSON(http.StatusOK, servicedto.StorageInfo{})
			return
		}
		info, err := provider.GetStorageInfo(c.Request.Context())
		if err != nil {
			writeInternalError(c, "get storage info failed", err)
			return
		}
		c.JSON(http.StatusOK, info)
	})

	router.POST("/settings/storage/backups", func(c *gin.Context) {
		if provider == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "database settings provider is not configured"})
			return
		}
		var request createBackupRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result, err := provider.CreateBackup(c.Request.Context(), servicedto.CreateBackupInput{
			RequestLogs:     request.RequestLogs,
			UsageLogs:       request.UsageLogs,
			UsageIdentities: request.UsageIdentities,
			APIKeys:         request.APIKeys,
			RedisInbox:      request.RedisInbox,
			ModelPrices:     request.ModelPrices,
		})
		if err != nil {
			if errors.Is(err, service.ErrStorageBackupDomainNeeded) || errors.Is(err, service.ErrDatabaseBackupsUnsupported) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			writeInternalError(c, "create database backup failed", err)
			return
		}
		c.JSON(http.StatusOK, result)
	})

	router.POST("/settings/storage/restore", func(c *gin.Context) {
		if provider == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "database settings provider is not configured"})
			return
		}
		var request restoreBackupRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result, err := provider.RestoreBackup(c.Request.Context(), servicedto.RestoreBackupInput{
			ID:               request.ID,
			RequestLogs:      request.RequestLogs,
			UsageLogs:        request.UsageLogs,
			UsageIdentities:  request.UsageIdentities,
			APIKeys:          request.APIKeys,
			RedisInbox:       request.RedisInbox,
			ModelPrices:      request.ModelPrices,
			SkipSafetyBackup: request.SkipSafetyBackup,
		})
		if err != nil {
			if errors.Is(err, service.ErrStorageRestoreDomainNeeded) || errors.Is(err, service.ErrStorageBackupNotFound) || errors.Is(err, service.ErrDatabaseBackupsUnsupported) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			writeInternalError(c, "restore database backup failed", err)
			return
		}
		c.JSON(http.StatusOK, result)
	})
}

func mapDatabaseSettingsInput(request updateDatabaseCleanupSettingsRequest) servicedto.UpdateDatabaseCleanupSettingsInput {
	return servicedto.UpdateDatabaseCleanupSettingsInput{
		RecordRequestDetails:    request.RecordRequestDetails,
		CleanupRequestLogs:      request.CleanupRequestLogs,
		CleanupUsageLogs:        request.CleanupUsageLogs,
		RequestLogRetentionDays: request.RequestLogRetentionDays,
		UsageLogRetentionDays:   request.UsageLogRetentionDays,
		MaxDatabaseSizeMB:       request.MaxDatabaseSizeMB,
		BackupRequestLogs:       request.BackupRequestLogs,
		BackupUsageLogs:         request.BackupUsageLogs,
		BackupUsageIdentities:   request.BackupUsageIdentities,
		BackupAPIKeys:           request.BackupAPIKeys,
		BackupRedisInbox:        request.BackupRedisInbox,
		BackupModelPrices:       request.BackupModelPrices,
		BackupHour:              request.BackupHour,
		BackupMinute:            request.BackupMinute,
		MaxBackupCount:          request.MaxBackupCount,
	}
}

func mapDatabaseSettingsResponse(snapshot service.DatabaseCleanupSettingsSnapshot) databaseCleanupSettingsResponse {
	settings := snapshot.Settings
	return databaseCleanupSettingsResponse{
		RecordRequestDetails:     settings.RecordRequestDetails,
		CleanupRequestLogs:       settings.CleanupRequestLogs,
		CleanupUsageLogs:         settings.CleanupUsageLogs,
		RequestLogRetentionDays:  settings.RequestLogRetentionDays,
		UsageLogRetentionDays:    settings.UsageLogRetentionDays,
		MaxDatabaseSizeMB:        settings.MaxDatabaseSizeMB,
		BackupRequestLogs:        settings.BackupRequestLogs,
		BackupUsageLogs:          settings.BackupUsageLogs,
		BackupUsageIdentities:    settings.BackupUsageIdentities,
		BackupAPIKeys:            settings.BackupAPIKeys,
		BackupRedisInbox:         settings.BackupRedisInbox,
		BackupModelPrices:        settings.BackupModelPrices,
		BackupHour:               settings.BackupHour,
		BackupMinute:             settings.BackupMinute,
		MaxBackupCount:           settings.MaxBackupCount,
		CurrentDatabaseSizeBytes: snapshot.CurrentDatabaseSizeBytes,
	}
}
