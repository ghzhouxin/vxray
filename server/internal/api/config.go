package api

import (
	"github.com/gin-gonic/gin"
	"v2ray-server/internal/config"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"
)

type ConfigHandler struct{ services *service.Container }

func NewConfigHandler(services *service.Container) *ConfigHandler {
	return &ConfigHandler{services: services}
}

func (h *ConfigHandler) Get(c *gin.Context) {
	response.Success(c, dto.ConfigResponse{
		Settings: toUserSettingsDTO(h.services.Config.UserSettings()),
		System:   toSystemMetaDTO(h.services.Config.SystemMeta()),
	})
}

func (h *ConfigHandler) GetDefault(c *gin.Context) {
	response.Success(c, toUserSettingsDTO(config.DefaultUserSettings()))
}

func (h *ConfigHandler) Update(c *gin.Context) {
	var req dto.UserSettingsDTO
	if !bindJSON(c, &req) {
		return
	}
	settings := toUserSettings(req)
	if handleError(c, h.services.Config.UpdateAndSaveSettings(settings)) {
		return
	}
	response.SuccessMessage(c, "config updated")
}
