package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"
)

type SubscriptionHandler struct{ services *service.Container }

func NewSubscriptionHandler(services *service.Container) *SubscriptionHandler {
	return &SubscriptionHandler{services: services}
}

func (h *SubscriptionHandler) List(c *gin.Context) {
	subs, err := h.services.Subscription.List()
	if handleError(c, err) {
		return
	}
	response.Success(c, toSubscriptionDTOs(subs))
}

func (h *SubscriptionHandler) Create(c *gin.Context) {
	var req dto.SubscriptionRequest
	if !bindJSON(c, &req) {
		return
	}
	sub := &model.Subscription{Name: req.Name, URL: req.URL}
	if handleError(c, h.services.Subscription.Create(sub)) {
		return
	}
	response.Created(c, toSubscriptionDTO(*sub))
}

func (h *SubscriptionHandler) Update(c *gin.Context) {
	var req dto.SubscriptionRequest
	if !bindJSON(c, &req) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	sub := &model.Subscription{ID: id, Name: req.Name, URL: req.URL}
	updated, err := h.services.Subscription.Update(sub)
	if handleError(c, err) {
		return
	}
	response.Success(c, toSubscriptionDTO(*updated))
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if handleError(c, h.services.Subscription.Delete(id)) {
		return
	}
	response.SuccessMessage(c, "deleted")
}

func (h *SubscriptionHandler) RefreshNodes(c *gin.Context) {
	var req dto.SubscriptionBatchUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		h.streamBatchRefresh(c, req.IDs)
		return
	}
	result, err := h.services.Subscription.UpdateNodesBatch(c.Request.Context(), req.IDs)
	if handleError(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *SubscriptionHandler) streamBatchRefresh(c *gin.Context, ids []uint) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ctx := c.Request.Context()
	progressChan, unsubscribe := h.services.Subscription.SubscribeBatchProgress()
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = h.services.Subscription.UpdateNodesBatch(ctx, ids)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case progress, ok := <-progressChan:
			if !ok {
				return
			}
			if err := writeSSE(c, progress); err != nil {
				return
			}
		}
	}
}

func writeSSE(c *gin.Context, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		_, _ = c.Writer.WriteString("\n")
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}
