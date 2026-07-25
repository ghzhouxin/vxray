package api

import (
	"v2ray-server/internal/config"
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

	ports, err := h.services.Xray.GetXrayPorts()
	if err != nil {
		_ = h.services.Log.Error(constants.TagXray, "获取 Xray 端口失败", map[string]any{"error": err.Error()})
	}
	proxy := dto.ProxyDTO{Enabled: h.services.Proxy.IsEnabled()}
	if ports != nil {
		proxy.HTTPPort = ports.HTTPPort
		proxy.SOCKSPort = ports.SOCKSPort
	}

	runtime := dto.ConsoleRuntimeDTO{
		Running: h.services.Xray.Status(),
		Proxy:   proxy,
		Ports:   dto.PortsDTO{HTTP: proxy.HTTPPort, SOCKS: proxy.SOCKSPort},
	}
	if node := h.services.Xray.GetActiveNode(); node != nil {
		runtime.CurrentNode = toNodeInfo(node)
	}

	cachedTargets, err := h.services.Config.GetWebsiteSpeedTestResults()
	if handleError(c, err) {
		return
	}
	websiteTargets := h.services.Config.UserSettings().SpeedTest.WebsiteTargets
	speedTestTargets := mergeWebsiteSpeedTestTargets(websiteTargets, cachedTargets)

	response.Success(c, dto.ConsoleSnapshot{
		NodeSummary:      *nodeSummary,
		Runtime:          runtime,
		SpeedTestTargets: speedTestTargets,
		Subscriptions:    toSubscriptionDTOs(subscriptions),
		Protocols:        protocols,
		Logs: dto.ConsoleLogsDTO{
			Items:      toLogDTOs(logs),
			Tags:       tags,
			Levels:     levels,
			NextCursor: nextCursor,
			HasMore:    nextCursor != "",
		},
	})
}

func mergeWebsiteSpeedTestTargets(targets []config.SpeedTestTarget, cached []config.WebsiteSpeedTestResult) []dto.WebsiteSpeedTestResultDTO {
	cachedByURL := make(map[string]config.WebsiteSpeedTestResult, len(cached))
	for _, item := range cached {
		if item.URL != "" {
			cachedByURL[item.URL] = item
		}
	}

	results := make([]dto.WebsiteSpeedTestResultDTO, len(targets))
	for i, target := range targets {
		item := dto.WebsiteSpeedTestResultDTO{Name: target.Name, URL: target.URL}
		if c, ok := cachedByURL[target.URL]; ok {
			item.Latency = c.Latency
			item.Error = c.Error
		}
		results[i] = item
	}
	return results
}
