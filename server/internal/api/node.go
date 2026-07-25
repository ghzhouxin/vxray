package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"
)

type NodeHandler struct{ services *service.Container }

func NewNodeHandler(services *service.Container) *NodeHandler {
	return &NodeHandler{services: services}
}

func (h *NodeHandler) List(c *gin.Context) {
	filter := buildNodeFilter(c)
	nodes, nextCursor, err := h.services.Node.List(filter)
	if handleError(c, err) {
		return
	}
	response.Success(c, dto.NodeListResponse{Items: service.ToNodeInfos(nodes), NextCursor: nextCursor})
}

func (h *NodeHandler) SetActive(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if handleError(c, h.services.Node.SetActive(id)) {
		return
	}
	response.SuccessMessage(c, "node activated")
}

func (h *NodeHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if handleError(c, h.services.Node.Delete(id)) {
		return
	}
	response.SuccessMessage(c, "node deleted")
}

func (h *NodeHandler) DeleteFailed(c *gin.Context) {
	var req dto.NodeActionFilterRequest
	if !bindJSON(c, &req) {
		return
	}

	filter := buildNodeActionFilter(req)
	var count int64
	var err error
	if isEmptyNodeActionFilter(filter) {
		count, err = h.services.Node.DeleteFailedNodes()
	} else {
		count, err = h.services.Node.DeleteFailedNodesByFilter(filter)
	}
	if handleError(c, err) {
		return
	}
	response.Success(c, gin.H{"count": count})
}

func (h *NodeHandler) SpeedTestByFilter(c *gin.Context) {
	var req dto.NodeSpeedTestRequest
	if !bindJSON(c, &req) {
		return
	}

	selection := service.NodeSpeedTestSelection{
		IDs:    req.IDs,
		Filter: buildNodeActionFilter(req.Filter),
	}

	if _, err := h.services.Node.StartSpeedTestJob(selection); err != nil {
		if errors.Is(err, service.ErrNodeSpeedTestRunning) {
			response.Conflict(c, "已有节点测速任务执行中", h.services.Node.SpeedTestStatus())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	h.streamSpeedTest(c)
}

func (h *NodeHandler) SpeedTestStatus(c *gin.Context) {
	response.Success(c, h.services.Node.SpeedTestStatus())
}

func (h *NodeHandler) streamSpeedTest(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ctx := c.Request.Context()
	progressChan, unsubscribe := h.services.Node.SubscribeSpeedTestProgress()
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case progress, ok := <-progressChan:
			if !ok {
				return
			}
			data, _ := json.Marshal(progress)
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

func buildNodeActionFilter(req dto.NodeActionFilterRequest) model.NodeFilter {
	return model.NodeFilter{
		SubscriptionID:  req.SubscriptionID,
		Protocol:        req.Protocol,
		Keyword:         req.Keyword,
		LatencyStatuses: req.LatencyStatuses,
	}
}

func isEmptyNodeActionFilter(filter model.NodeFilter) bool {
	return filter.SubscriptionID == 0 &&
		filter.Protocol == "" &&
		filter.Keyword == "" &&
		len(filter.LatencyStatuses) == 0
}
