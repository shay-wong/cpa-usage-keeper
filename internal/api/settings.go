package api

import (
	"net/http"
	"strings"

	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"github.com/gin-gonic/gin"
)

type databaseCleanupSettingsResponse struct {
	RequestLogRetentionDays int `json:"request_log_retention_days"`
	MaxDatabaseSizeMB       int `json:"max_database_size_mb"`
}

type updateDatabaseCleanupSettingsRequest struct {
	RequestLogRetentionDays int `json:"request_log_retention_days"`
	MaxDatabaseSizeMB       int `json:"max_database_size_mb"`
}

func registerSettingsRoutes(router gin.IRoutes, provider service.DatabaseSettingsProvider) {
	router.GET("/settings/database", func(c *gin.Context) {
		if provider == nil {
			c.JSON(http.StatusOK, databaseCleanupSettingsResponse{})
			return
		}
		settings, err := provider.GetDatabaseCleanupSettings(c.Request.Context())
		if err != nil {
			writeInternalError(c, "get database cleanup settings failed", err)
			return
		}
		c.JSON(http.StatusOK, databaseCleanupSettingsResponse{
			RequestLogRetentionDays: settings.RequestLogRetentionDays,
			MaxDatabaseSizeMB:       settings.MaxDatabaseSizeMB,
		})
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
		if request.RequestLogRetentionDays < 0 || request.MaxDatabaseSizeMB < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "database cleanup settings must be non-negative"})
			return
		}
		settings, err := provider.UpdateDatabaseCleanupSettings(c.Request.Context(), servicedto.UpdateDatabaseCleanupSettingsInput{
			RequestLogRetentionDays: request.RequestLogRetentionDays,
			MaxDatabaseSizeMB:       request.MaxDatabaseSizeMB,
		})
		if err != nil {
			if strings.Contains(err.Error(), "non-negative") {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			writeInternalError(c, "update database cleanup settings failed", err)
			return
		}
		c.JSON(http.StatusOK, databaseCleanupSettingsResponse{
			RequestLogRetentionDays: settings.RequestLogRetentionDays,
			MaxDatabaseSizeMB:       settings.MaxDatabaseSizeMB,
		})
	})
}
