package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"v2ray-server/internal/config"
	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"
	"v2ray-server/pkg/speedtest"
	"v2ray-server/pkg/subscription"
	"v2ray-server/pkg/types"
	"v2ray-server/pkg/xray"

	"gorm.io/gorm"
)

// NodeInfo 是 model.Node 的服务层投影，补充 ProtocolLabel 等派生字段。
type NodeInfo struct {
	ID             uint            `json:"id"`
	SubscriptionID uint            `json:"subscription_id"`
	Name           string          `json:"name"`
	Protocol       string          `json:"protocol"`
	ProtocolLabel  string          `json:"protocol_label"`
	Address        string          `json:"address"`
	Port           int             `json:"port"`
	RawURL         string          `json:"raw_url"`
	Transport      types.Transport `json:"transport"`
	Latency        int64           `json:"latency"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// NodeSpeedTestStatus 是节点测速任务的当前状态。
type NodeSpeedTestStatus struct {
	Running    bool               `json:"running"`
	Progress   *OperationProgress `json:"progress,omitempty"`
	StartedAt  string             `json:"started_at,omitempty"`
	FinishedAt string             `json:"finished_at,omitempty"`
	Error      string             `json:"error,omitempty"`
}

// NodeSummary 按延迟状态统计节点数量。
type NodeSummary struct {
	All       int64 `json:"all"`
	Available int64 `json:"available"`
	Pending   int64 `json:"pending"`
	Timeout   int64 `json:"timeout"`
}

// ProtocolOption 是协议筛选的下拉选项。
type ProtocolOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type NodeService struct {
	repo      *repository.NodeRepository
	speedTest *speedtest.SpeedTest
	xraySvc   *XrayService
	cfg       *config.State
	logSvc    *LogService
	speedMu   sync.Mutex
	speedBus  *ProgressBus
}

const progressChanBuffer = 100

type NodeSpeedTestSelection struct {
	IDs    []uint
	Filter model.NodeFilter
}

var ErrNodeSpeedTestRunning = errors.New("node_speedtest_running")

func NewNodeService(xraySvc *XrayService, repo *repository.NodeRepository, cfg *config.State, logger *LogService) *NodeService {
	return &NodeService{
		repo:      repo,
		speedTest: speedtest.New(cfg),
		xraySvc:   xraySvc,
		cfg:       cfg,
		logSvc:    logger,
	}
}

// MigrateEmptyTransportNodes re-parses nodes with empty Transport from their stored RawURL.
// Handles nodes persisted before the Transport refactor so Xray can build valid outbounds.
func (s *NodeService) MigrateEmptyTransportNodes() error {
	nodes, err := s.repo.FindByEmptyTransport()
	if err != nil {
		return fmt.Errorf("find nodes with empty transport: %w", err)
	}
	if len(nodes) == 0 {
		return nil
	}

	s.logSvc.Info(constants.TagSubscription, "迁移空 Transport 节点", map[string]any{"count": len(nodes)})
	migrated, failed := 0, 0
	for _, node := range nodes {
		parsed, err := subscription.Parse(node.RawURL)
		if err != nil {
			failed++
			s.logSvc.Info(constants.TagSubscription, "重新解析节点失败，跳过", map[string]any{"id": node.ID, "error": err.Error()})
			continue
		}
		if err := s.repo.UpdateTransport(node.ID, parsed.Transport); err != nil {
			failed++
			s.logSvc.Error(constants.TagSubscription, "更新节点 Transport 失败", map[string]any{"id": node.ID, "error": err.Error()})
			continue
		}
		migrated++
	}
	s.logSvc.Info(constants.TagSubscription, "Transport 迁移完成", map[string]any{"migrated": migrated, "failed": failed})
	return nil
}

// RestoreActiveOutbound 重建活跃节点的 outbound 并更新 config 文件。
// 用于迁移后修复 stale config 文件，确保 xray 启动时使用正确的 outbound。
func (s *NodeService) RestoreActiveOutbound() error {
	activeID := s.cfg.ActiveNodeID()
	if activeID == 0 {
		return nil
	}
	node, err := s.repo.FindByID(activeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 节点已随订阅删除，清空持久化的 active id
			_ = s.cfg.SetActiveNodeID(0)
			return nil
		}
		return fmt.Errorf("find active node %d: %w", activeID, err)
	}
	parsed, err := subscription.Parse(node.RawURL)
	if err != nil {
		return fmt.Errorf("parse active node %d: %w", activeID, err)
	}
	outbound := xray.BuildOutbound(parsed)
	return s.cfg.UpdateXrayOutbound(outbound)
}

func (s *NodeService) List(filter model.NodeFilter) ([]*model.Node, string, error) {
	if filter.Cursor == "" {
		if nodes, nextCursor, ok, err := s.listWithPinnedActiveNode(filter); ok || err != nil {
			return nodes, nextCursor, err
		}
	}
	return s.repo.FindByFilter(filter)
}

func (s *NodeService) Get(id uint) (*model.Node, error) {
	return s.repo.FindByID(id)
}

func (s *NodeService) StartSpeedTestJob(selection NodeSpeedTestSelection) (*NodeSpeedTestStatus, error) {
	s.speedMu.Lock()
	if s.speedBus != nil && s.speedBus.IsRunning() {
		status := s.speedTestStatus()
		s.speedMu.Unlock()
		return status, ErrNodeSpeedTestRunning
	}

	bus := NewProgressBus()
	bus.Start()
	bus.Publish(OperationProgress{
		Type:    "node_speedtest",
		Status:  "running",
		Stage:   "prepare",
		Message: "准备节点测速",
	}, false)
	s.speedBus = bus
	status := s.speedTestStatus()
	s.speedMu.Unlock()

	go s.runSpeedTestJob(selection, bus)
	return status, nil
}

func (s *NodeService) SpeedTestStatus() *NodeSpeedTestStatus {
	s.speedMu.Lock()
	defer s.speedMu.Unlock()
	return s.speedTestStatus()
}

func (s *NodeService) speedTestStatus() *NodeSpeedTestStatus {
	if s.speedBus == nil {
		return &NodeSpeedTestStatus{Running: false}
	}
	status := &NodeSpeedTestStatus{
		Running:   s.speedBus.IsRunning(),
		Progress:  s.speedBus.Last(),
		StartedAt: formatOptionalTime(s.speedBus.StartedAt()),
	}
	if finishedAt := s.speedBus.FinishedAt(); !finishedAt.IsZero() {
		status.FinishedAt = finishedAt.Format(time.RFC3339)
	}
	if status.Running && status.Progress != nil && status.Progress.Error != "" {
		status.Error = status.Progress.Error
	}
	return status
}

func (s *NodeService) SubscribeSpeedTestProgress() (<-chan OperationProgress, func()) {
	s.speedMu.Lock()
	bus := s.speedBus
	s.speedMu.Unlock()

	if bus == nil {
		ch := make(chan OperationProgress)
		close(ch)
		return ch, func() {}
	}
	return bus.Subscribe()
}

func (s *NodeService) runSpeedTestJob(selection NodeSpeedTestSelection, bus *ProgressBus) {
	var wrappedChan chan speedtest.Progress
	defer func() {
		if r := recover(); r != nil {
			if wrappedChan != nil {
				close(wrappedChan)
			}
			message := fmt.Sprintf("节点测速异常: %v", r)
			s.logSvc.Error(constants.TagSpeedtest, "节点测速异常", map[string]any{"error": message})
			bus.Publish(OperationProgress{
				Type:    "node_speedtest",
				Status:  "failed",
				Stage:   "panic",
				Error:   message,
				Message: "节点测速异常",
			}, true)
		}
	}()

	s.logSvc.Info(constants.TagSpeedtest, "准备节点测速", map[string]any{"ids": selection.IDs, "filter": selection.Filter})

	nodes, err := s.resolveSpeedTestNodes(selection)
	if err != nil {
		s.logSvc.Error(constants.TagSpeedtest, "节点测速准备失败", map[string]any{"error": err.Error()})
		bus.Publish(OperationProgress{
			Type:    "node_speedtest",
			Status:  "failed",
			Stage:   "prepare",
			Error:   err.Error(),
			Message: "节点测速准备失败",
		}, true)
		return
	}

	if len(nodes) == 0 {
		s.logSvc.Info(constants.TagSpeedtest, "当前筛选无可测速节点", map[string]any{"total": 0})
		bus.Publish(OperationProgress{
			Type:    "node_speedtest",
			Status:  "empty",
			Stage:   "empty",
			Message: "当前筛选无可测速节点",
		}, true)
		return
	}

	s.logSvc.Info(constants.TagSpeedtest, "开始节点测速", map[string]any{"total": len(nodes)})
	bus.Publish(OperationProgress{
		Type:    "node_speedtest",
		Status:  "running",
		Stage:   "start",
		Total:   len(nodes),
		Message: "开始节点测速",
	}, false)

	wrappedChan = s.createJobProgressWrapper(bus)
	s.speedTest.TestNodes(toSpeedNodes(nodes), wrappedChan)
}

func (s *NodeService) resolveSpeedTestNodes(selection NodeSpeedTestSelection) ([]*model.Node, error) {
	ids := uniqueUintIDs(selection.IDs)
	if len(ids) == 0 {
		var err error
		ids, err = s.repo.FindIDsByFilter(selection.Filter)
		if err != nil {
			return nil, err
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.repo.FindByIDs(ids)
}

func uniqueUintIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func (s *NodeService) createJobProgressWrapper(bus *ProgressBus) chan speedtest.Progress {
	wrappedChan := make(chan speedtest.Progress, progressChanBuffer)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				message := fmt.Sprintf("节点测速进度处理异常: %v", r)
				s.logSvc.Error(constants.TagSpeedtest, "节点测速异常", map[string]any{"error": message})
				bus.Publish(OperationProgress{
					Type:    "node_speedtest",
					Status:  "failed",
					Stage:   "panic",
					Error:   message,
					Message: "节点测速异常",
				}, true)
				return
			}
			// 正常结束：标记总线为终止状态，防止前端刷新后误判为“测速任务恢复中”
			if last := bus.Last(); last != nil {
				bus.Publish(*last, true)
			}
		}()

		const batchWriteSize = 10
		pending := make([]repository.LatencyUpdate, 0, batchWriteSize)
		flush := func() {
			if len(pending) > 0 {
				if err := s.repo.BatchUpdateLatency(pending); err != nil {
					s.logSvc.Error(constants.TagSpeedtest, "批量更新节点延迟失败", map[string]any{"count": len(pending), "error": err.Error()})
				}
				pending = pending[:0]
			}
		}
		defer flush()

		for p := range wrappedChan {
			if p.NodeID > 0 && !p.Testing {
				pending = append(pending, repository.LatencyUpdate{ID: p.NodeID, Latency: p.Latency})
				if len(pending) >= batchWriteSize {
					flush()
				}
			}

			status := "running"
			stage := "node_testing"
			message := "节点测速中"
			if !p.Testing {
				stage = "node_done"
				message = "节点测速完成"
			}

			if !p.Testing && p.Total > 0 && p.Completed == p.Total {
				status = "success"
				stage = "finished"
				message = "节点测速完成"
				if p.Success == 0 && p.Failed > 0 {
					status = "failed"
					message = "节点测速失败"
					s.logSvc.Error(constants.TagSpeedtest, "节点测速失败", speedTestProgressDetail(p))
				} else {
					s.logSvc.Info(constants.TagSpeedtest, "节点测速完成", speedTestProgressDetail(p))
				}
			}

			bus.Publish(OperationProgress{
				Type:      "node_speedtest",
				Status:    status,
				Stage:     stage,
				Total:     p.Total,
				Completed: p.Completed,
				Success:   p.Success,
				Failed:    p.Failed,
				NodeID:    p.NodeID,
				Latency:   p.Latency,
				Error:     p.ErrMsg,
				Message:   message,
				Testing:   p.Testing,
			}, false)
		}
	}()
	return wrappedChan
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func speedTestProgressDetail(p speedtest.Progress) map[string]any {
	return map[string]any{
		"total":     p.Total,
		"completed": p.Completed,
		"success":   p.Success,
		"failed":    p.Failed,
	}
}

func ToNodeInfos(nodes []*model.Node) []NodeInfo {
	infos := make([]NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		infos = append(infos, NodeInfo{
			ID:             node.ID,
			SubscriptionID: node.SubscriptionID,
			Name:           node.Name,
			Protocol:       node.Protocol,
			ProtocolLabel:  types.ProtocolLabel(node.Protocol),
			Address:        node.Address,
			Port:           node.Port,
			RawURL:         node.RawURL,
			Transport:      node.Transport,
			Latency:        node.Latency,
			CreatedAt:      node.CreatedAt,
			UpdatedAt:      node.UpdatedAt,
		})
	}
	return infos
}

func (s *NodeService) SetActive(id uint) error {
	node, err := s.Get(id)
	if err != nil {
		return err
	}
	outbound := xray.BuildOutbound(nodeToParsed(node))
	return s.xraySvc.SetActiveNode(id, outbound)
}

func nodeToParsed(n *model.Node) *types.ParsedNode {
	return &types.ParsedNode{
		Name:      n.Name,
		Protocol:  n.Protocol,
		Address:   n.Address,
		Port:      n.Port,
		RawConfig: n.RawConfig,
		Transport: n.Transport,
	}
}

func (s *NodeService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *NodeService) DeleteFailedNodes() (int64, error) {
	return s.repo.DeleteByLatency(constants.LatencyTimeout)
}

func (s *NodeService) DeleteFailedNodesByFilter(filter model.NodeFilter) (int64, error) {
	return s.repo.DeleteByFilterAndLatency(filter, constants.LatencyTimeout)
}

func (s *NodeService) GetTopNodes(limit int) ([]*model.Node, error) {
	return s.repo.FindTopNodes(limit)
}

func (s *NodeService) Summary() (*NodeSummary, error) {
	counts, err := s.repo.CountByLatencyStatus()
	if err != nil {
		return nil, err
	}
	return &NodeSummary{
		All:       counts.All,
		Available: counts.Available,
		Pending:   counts.Pending,
		Timeout:   counts.Timeout,
	}, nil
}

func (s *NodeService) GetProtocols() []ProtocolOption {
	options := make([]ProtocolOption, len(constants.NodeProtocols))
	for i, p := range constants.NodeProtocols {
		options[i] = ProtocolOption{Value: p, Label: types.ProtocolLabel(p)}
	}
	return options
}

func toSpeedNodes(nodes []*model.Node) []speedtest.Node {
	speedNodes := make([]speedtest.Node, len(nodes))
	for i, n := range nodes {
		speedNodes[i] = speedtest.Node{
			ID:       n.ID,
			Outbound: xray.BuildOutbound(nodeToParsed(n)),
		}
	}
	return speedNodes
}

func (s *NodeService) listWithPinnedActiveNode(filter model.NodeFilter) ([]*model.Node, string, bool, error) {
	activeID := s.cfg.ActiveNodeID()
	if activeID == 0 {
		return nil, "", false, nil
	}

	activeNode, err := s.repo.FindByID(activeID)
	if err != nil || activeNode == nil || !nodeMatchesFilter(activeNode, filter) {
		return nil, "", false, nil
	}

	pinnedFilter := filter
	pinnedFilter.ExcludeID = activeID
	if filter.Limit > 0 {
		if filter.Limit == 1 {
			pinnedFilter.Limit = 1
			_, nextCursor, err := s.repo.FindByFilter(pinnedFilter)
			if err != nil {
				return nil, "", true, err
			}
			return []*model.Node{activeNode}, nextCursor, true, nil
		}
		pinnedFilter.Limit = filter.Limit - 1
	}
	nodes, nextCursor, err := s.repo.FindByFilter(pinnedFilter)
	if err != nil {
		return nil, "", true, err
	}

	pinned := make([]*model.Node, 0, len(nodes)+1)
	pinned = append(pinned, activeNode)
	pinned = append(pinned, nodes...)
	return pinned, nextCursor, true, nil
}

func nodeMatchesFilter(node *model.Node, filter model.NodeFilter) bool {
	if filter.SubscriptionID > 0 && node.SubscriptionID != filter.SubscriptionID {
		return false
	}
	if filter.Protocol != "" && node.Protocol != filter.Protocol {
		return false
	}
	if filter.Keyword != "" && !nodeMatchesKeyword(node, filter.Keyword) {
		return false
	}
	if len(filter.LatencyStatuses) == 0 {
		return true
	}
	for _, status := range filter.LatencyStatuses {
		if constants.LatencyStatus(node.Latency) == status {
			return true
		}
	}
	return false
}

// nodeMatchesKeyword 调用方保证 node 非 nil、keyword 非空。
func nodeMatchesKeyword(node *model.Node, keyword string) bool {
	lowerKeyword := strings.ToLower(keyword)
	contains := func(text string) bool {
		return strings.Contains(strings.ToLower(text), lowerKeyword)
	}
	if contains(node.Name) || contains(node.Address) || contains(node.Protocol) || contains(node.RawURL) {
		return true
	}
	portStr := strconv.Itoa(node.Port)
	return portStr == keyword || contains(portStr)
}

