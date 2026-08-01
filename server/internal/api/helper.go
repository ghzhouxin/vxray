package api

import (
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"v2ray-server/internal/model"
	"v2ray-server/pkg/response"
)

func parseUintParam(c *gin.Context, key string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(key), 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid "+key)
		return 0, false
	}
	return uint(id), true
}

func parseUintQuery(c *gin.Context, key string) uint {
	if id, err := strconv.ParseUint(c.Query(key), 10, 32); err == nil {
		return uint(id)
	}
	return 0
}

func parseIntQuery(c *gin.Context, key string) int {
	if val, err := strconv.Atoi(c.Query(key)); err == nil && val > 0 {
		return val
	}
	return 0
}

func handleError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "resource not found")
		return true
	}
	log.Printf("api: internal error: %v", err)
	response.InternalError(c, "internal error")
	return true
}

func bindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		response.BadRequest(c, err.Error())
		return false
	}
	return true
}

func bindQuery(c *gin.Context, req any) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		response.BadRequest(c, err.Error())
		return false
	}
	return true
}

func buildNodeFilter(c *gin.Context) model.NodeFilter {
	return model.NodeFilter{
		Protocol:        c.Query("protocol"),
		Keyword:         c.Query("keyword"),
		LatencyStatuses: c.QueryArray("latency_statuses"),
		SubscriptionID:  parseUintQuery(c, "subscription_id"),
		Cursor:          c.Query("cursor"),
		Limit:           parseIntQuery(c, "limit"),
	}
}

func parseOptionalTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
