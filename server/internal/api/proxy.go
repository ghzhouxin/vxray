package api

import (
	"github.com/gin-gonic/gin"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"
)

type ProxyHandler struct {
	services *service.Container
}

func NewProxyHandler(services *service.Container) *ProxyHandler {
	return &ProxyHandler{services: services}
}

func (h *ProxyHandler) Toggle(c *gin.Context) {
	var req dto.ProxyToggleRequest
	if !bindJSON(c, &req) {
		return
	}
	msg, err := h.services.Proxy.Toggle(req.Enabled)
	if handleError(c, err) {
		return
	}
	response.SuccessMessage(c, msg)
}
