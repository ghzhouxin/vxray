package api

import (
	"v2ray-server/internal/constants"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"

	"github.com/gin-gonic/gin"
)

type ConsoleHandler struct{ services *service.Container }

func NewConsoleHandler(services *service.Container) *ConsoleHandler {
	return &ConsoleHandler{services: services}
}

func (h *ConsoleHandler) Get(c *gin.Context) {
	nodeSummary, err := h.services.Node.Summary()
	if handleError(c, err) {
		return
	}

	protocols := h.services.Node.GetProtocols()

	subscriptions, err := h.services.Subscription.List()
	if handleError(c, err) {
		return
	}

	logs, nextCursor, err := h.services.Log.List(model.LogFilter{Limit: constants.DefaultLogPageSize})
	if handleError(c, err) {
		return
	}

	tags := h.services.Log.GetTags()
	levels := h.services.Log.GetLevels()

	ports, err := h.services.Xray.XrayPorts()
	if err != nil {
		_ = h.services.Log.Error(constants.TagXray, "获取 Xray 端口失败", map[string]any{"error": err.Error()})
	}
	proxy := dto.ProxyDTO{Enabled: h.services.Proxy.IsEnabled()}
	if ports != nil {
		proxy.HTTPPort = ports.HTTPPort
		proxy.SOCKSPort = ports.SOCKSPort
	}

	tunState := h.services.Tun.Status()
	runtime := dto.ConsoleRuntimeDTO{
		Running:    h.services.Xray.Status(),
		TunEnabled: tunState == service.TunEnabled,
		TunState:   tunState.String(),
		Proxy:      proxy,
	}
	if node := h.services.Xray.GetActiveNode(); node != nil {
		runtime.CurrentNode = toNodeInfo(node)
	}

	response.Success(c, dto.ConsoleSnapshot{
		NodeSummary:   *nodeSummary,
		Runtime:       runtime,
		Subscriptions: toSubscriptionDTOs(subscriptions),
		Protocols:     protocols,
		Logs: dto.ConsoleLogsDTO{
			Items:      toLogDTOs(logs),
			Tags:       tags,
			Levels:     levels,
			NextCursor: nextCursor,
			HasMore:    nextCursor != "",
		},
	})
}
