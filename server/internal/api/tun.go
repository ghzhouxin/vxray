package api

import (
	"v2ray-server/internal/dto"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"

	"github.com/gin-gonic/gin"
)

type TunHandler struct{ services *service.Container }

func NewTunHandler(services *service.Container) *TunHandler {
	return &TunHandler{services: services}
}

func (h *TunHandler) Enable(c *gin.Context) {
	if handleError(c, h.services.Tun.Enable()) {
		return
	}
	response.SuccessMessage(c, "tun enabled")
}

func (h *TunHandler) Disable(c *gin.Context) {
	if handleError(c, h.services.Tun.Disable()) {
		return
	}
	response.SuccessMessage(c, "tun disabled")
}

func (h *TunHandler) Status(c *gin.Context) {
	state := h.services.Tun.Status()
	response.Success(c, dto.TunStatusResponse{
		Enabled: state == service.TunEnabled,
		State:   state.String(),
	})
}
