package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"v2ray-server/internal/constants"
	"v2ray-server/internal/service"
)

type ClashHandler struct{ services *service.Container }

func NewClashHandler(services *service.Container) *ClashHandler {
	return &ClashHandler{services: services}
}

func (h *ClashHandler) Subscription(c *gin.Context) {
	nodes, err := h.services.Node.GetTopNodes(constants.ClashTopNodesLimit)
	if handleError(c, err) {
		return
	}

	data, err := service.GenerateClashConfig(nodes)
	if handleError(c, err) {
		return
	}

	c.Data(http.StatusOK, "text/yaml; charset=utf-8", data)
}
