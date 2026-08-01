package api

import (
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
	result, err := h.services.Subscription.UpdateNodesBatch(c.Request.Context(), req.IDs)
	if handleError(c, err) {
		return
	}
	response.Success(c, result)
}
