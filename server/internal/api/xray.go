package api

import (
	"errors"

	"github.com/gin-gonic/gin"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"
)

type XrayHandler struct{ services *service.Container }

func NewXrayHandler(services *service.Container) *XrayHandler {
	return &XrayHandler{services: services}
}

func (h *XrayHandler) Runtime(c *gin.Context) {
	xraySvc := h.services.Xray
	resp := &dto.XrayStatusResponse{
		Running: xraySvc.Status(),
	}
	if node := xraySvc.GetActiveNode(); node != nil {
		resp.CurrentNode = toNodeInfo(node)
	}
	response.Success(c, resp)
}

func (h *XrayHandler) StartRuntime(c *gin.Context) {
	if handleError(c, h.services.Xray.Start()) {
		return
	}
	response.SuccessMessage(c, "xray started")
}

func (h *XrayHandler) StopRuntime(c *gin.Context) {
	if handleError(c, h.services.Xray.Stop()) {
		return
	}
	response.SuccessMessage(c, "xray stopped")
}

func (h *XrayHandler) GetConfig(c *gin.Context) {
	content, err := h.services.Xray.GetConfig()
	if handleError(c, err) {
		return
	}
	response.Success(c, gin.H{"content": content})
}

func (h *XrayHandler) GetDefaultConfig(c *gin.Context) {
	content, err := h.services.Xray.GetDefaultConfig()
	if handleError(c, err) {
		return
	}
	response.Success(c, gin.H{"content": content})
}

func (h *XrayHandler) SaveConfig(c *gin.Context) {
	var req dto.SaveConfigRequest
	if !bindJSON(c, &req) {
		return
	}
	if handleError(c, h.services.Xray.SaveConfig(req.Content)) {
		return
	}
	if err := h.services.Xray.Restart(); err != nil {
		response.Success(c, gin.H{"warning": "config saved, but restart failed: " + err.Error()})
		return
	}
	response.SuccessMessage(c, "config saved and xray started")
}

func (h *XrayHandler) SpeedTestWebsites(c *gin.Context) {
	if !h.services.Xray.Status() {
		response.BadRequest(c, "xray not running")
		return
	}
	ports, err := h.services.Xray.XrayPorts()
	if handleError(c, err) {
		return
	}
	if ports.SOCKSPort == 0 {
		response.BadRequest(c, "no socks port available")
		return
	}

	if err := h.services.Xray.SpeedTestWebsite(ports.SOCKSPort); err != nil {
		if errors.Is(err, service.ErrWebsiteSpeedTestRunning) {
			response.Conflict(c, "网站测速进行中", nil)
			return
		}
		handleError(c, err)
		return
	}
	response.Success(c, nil)
}
