package api

import (
	"context"
	"net/http"

	"cpa-usage-keeper/internal/updatecheck"
	"github.com/gin-gonic/gin"
)

type updateChecker interface {
	Check(context.Context) (updatecheck.Result, error)
}

func registerUpdateRoutes(router gin.IRoutes, checker updateChecker) {
	if checker == nil {
		checker = updatecheck.DefaultChecker()
	}

	router.GET("/update/check", func(c *gin.Context) {
		result, err := checker.Check(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusOK, updatecheck.Result{
				UpdateAvailable: false,
				CanCompare:      false,
				Message:         "update check unavailable",
				Error:           err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, result)
	})
}
