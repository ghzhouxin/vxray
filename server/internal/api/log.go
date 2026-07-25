package api

import (
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"

	"github.com/gin-gonic/gin"
)

type LogHandler struct{ services *service.Container }

func NewLogHandler(services *service.Container) *LogHandler { return &LogHandler{services: services} }

func (h *LogHandler) List(c *gin.Context) {
	var req dto.LogFilter
	if !bindQuery(c, &req) {
		return
	}

	filter := model.LogFilter{
		Level:     req.Level,
		Tag:       req.Tag,
		Keyword:   req.Keyword,
		Limit:     req.Limit,
		Cursor:    req.Cursor,
		StartTime: parseOptionalTime(req.StartTime),
		EndTime:   parseOptionalTime(req.EndTime),
	}

	logs, nextCursor, err := h.services.Log.List(filter)
	if handleError(c, err) {
		return
	}

	response.Success(c, dto.LogListResponse{
		Items:      toLogDTOs(logs),
		Tags:       h.services.Log.GetTags(),
		Levels:     h.services.Log.GetLevels(),
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

func (h *LogHandler) Clear(c *gin.Context) {
	deletedCount, err := h.services.Log.Clear()
	if handleError(c, err) {
		return
	}
	response.Success(c, gin.H{"message": "logs cleared", "deleted_count": deletedCount})
}
