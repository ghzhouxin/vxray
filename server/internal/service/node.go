package service

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"v2ray-server/internal/config"
	"v2ray-server/internal/constants"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"
	"v2ray-server/pkg/speedtest"
	"v2ray-server/pkg/types"
)

type NodeService struct {
	repo      *repository.NodeRepository
	speedTest *speedtest.SpeedTest
	xraySvc   *XrayService
	cfg       *config.State
	logger    *LogService
	speedMu   sync.Mutex
	speedJob  *nodeSpeedTestJob
}

const (
	progressChanBuffer       = 100
	batchProgressStepNode    = 10
	batchProgressStepPercent = 10
)

type NodeSpeedTestSelection struct {
	IDs    []uint
	Filter model.NodeFilter
}

type nodeSpeedTestJob struct {
	running      bool
	lastProgress *dto.OperationProgress
	targetNodes  []dto.NodeInfo
	startedAt    time.Time
	finishedAt   time.Time
	subscribers  map[chan dto.OperationProgress]struct{}
}

var ErrNodeSpeedTestRunning = errors.New("node_speedtest_running")

func NewNodeService(xraySvc *XrayService, repo *repository.NodeRepository, cfg *config.State, logger *LogService) *NodeService {
	return &NodeService{
		repo:      repo,
		speedTest: speedtest.New(cfg.UserSettings()),
		xraySvc:   xraySvc,
		cfg:       cfg,
		logger:    logger,
	}
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

func (s *NodeService) StartSpeedTestJob(selection NodeSpeedTestSelection) (*dto.NodeSpeedTestStatus, error) {
	s.speedMu.Lock()
	if s.speedJob != nil && s.speedJob.running {
		status := s.speedTestStatusLocked()
		s.speedMu.Unlock()
		return status, ErrNodeSpeedTestRunning
	}

	job := &nodeSpeedTestJob{
		running:     true,
		startedAt:   time.Now(),
		subscribers: make(map[chan dto.OperationProgress]struct{}),
	}
	job.lastProgress = &dto.OperationProgress{
		Type:    "node_speedtest",
		Status:  "running",
		Stage:   "prepare",
		Message: "准备节点测速",
	}
	s.speedJob = job
	status := s.speedTestStatusLocked()
	s.speedMu.Unlock()

	go s.runSpeedTestJob(selection, job)
	return status, nil
}

func (s *NodeService) SpeedTestStatus() *dto.NodeSpeedTestStatus {
	s.speedMu.Lock()
	defer s.speedMu.Unlock()
	return s.speedTestStatusLocked()
}

func (s *NodeService) SubscribeSpeedTestProgress() (<-chan dto.OperationProgress, func()) {
	ch := make(chan dto.OperationProgress, progressChanBuffer)
	s.speedMu.Lock()
	job := s.speedJob
	if job == nil {
		close(ch)
		s.speedMu.Unlock()
		return ch, func() {}
	}

	running := job.running
	lastProgress := cloneOperationProgress(job.lastProgress)
	if running {
		job.subscribers[ch] = struct{}{}
	}
	s.speedMu.Unlock()

	if lastProgress != nil {
		ch <- *lastProgress
	}
	if !running {
		close(ch)
	}

	return ch, func() {
		s.speedMu.Lock()
		if job := s.speedJob; job != nil && job.subscribers != nil {
			delete(job.subscribers, ch)
		}
		s.speedMu.Unlock()
	}
}

func (s *NodeService) runSpeedTestJob(selection NodeSpeedTestSelection, job *nodeSpeedTestJob) {
	var op *OperationLog
	var wrappedChan chan speedtest.Progress
	defer func() {
		if r := recover(); r != nil {
			if wrappedChan != nil {
				close(wrappedChan)
			}
			message := fmt.Sprintf("节点测速异常: %v", r)
			if op != nil {
				s.failSpeedTestOperation(op, "节点测速异常", map[string]any{"error": message})
			}
			s.publishSpeedTestProgress(job, dto.OperationProgress{
				Type:    "node_speedtest",
				Status:  "failed",
				Stage:   "panic",
				Error:   message,
				Message: "节点测速异常",
			})
		}
	}()

	op = s.startSpeedTestOperation("准备节点测速", map[string]any{
		"ids":    selection.IDs,
		"filter": selection.Filter,
	})

	nodes, err := s.resolveSpeedTestNodes(selection)
	if err != nil {
		s.failSpeedTestOperation(op, "节点测速准备失败", map[string]any{"error": err.Error()})
		s.publishSpeedTestProgress(job, dto.OperationProgress{
			Type:    "node_speedtest",
			Status:  "failed",
			Stage:   "prepare",
			Error:   err.Error(),
			Message: "节点测速准备失败",
		})
		return
	}

	if len(nodes) == 0 {
		detail := map[string]any{"total": 0}
		s.finishSpeedTestOperation(op, "当前筛选无可测速节点", detail)
		s.publishSpeedTestProgress(job, dto.OperationProgress{
			Type:    "node_speedtest",
			Status:  "empty",
			Stage:   "empty",
			Message: "当前筛选无可测速节点",
		})
		return
	}

	nodeInfos := ToNodeInfos(nodes)
	s.updateSpeedTestOperation(op, "开始节点测速", map[string]any{"total": len(nodes)})
	s.publishSpeedTestProgress(job, dto.OperationProgress{
		Type:    "node_speedtest",
		Status:  "running",
		Stage:   "start",
		Total:   len(nodes),
		Message: "开始节点测速",
		Nodes:   nodeInfos,
	})

	wrappedChan = s.createJobProgressWrapper(job, op)
	s.speedTest.TestNodes(toSpeedNodes(nodes), wrappedChan)
}

func (s *NodeService) speedTestStatusLocked() *dto.NodeSpeedTestStatus {
	if s.speedJob == nil {
		return &dto.NodeSpeedTestStatus{Running: false}
	}

	status := &dto.NodeSpeedTestStatus{
		Running:   s.speedJob.running,
		Progress:  cloneOperationProgress(s.speedJob.lastProgress),
		Nodes:     append([]dto.NodeInfo(nil), s.speedJob.targetNodes...),
		StartedAt: formatOptionalTime(s.speedJob.startedAt),
	}
	if !s.speedJob.finishedAt.IsZero() {
		status.FinishedAt = s.speedJob.finishedAt.Format(time.RFC3339)
	}
	if status.Running && status.Progress != nil && status.Progress.Error != "" {
		status.Error = status.Progress.Error
	}
	return status
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

func (s *NodeService) createJobProgressWrapper(job *nodeSpeedTestJob, op *OperationLog) chan speedtest.Progress {
	wrappedChan := make(chan speedtest.Progress, progressChanBuffer)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				message := fmt.Sprintf("节点测速进度处理异常: %v", r)
				if op != nil {
					s.failSpeedTestOperation(op, "节点测速异常", map[string]any{"error": message})
				}
				s.publishSpeedTestProgress(job, dto.OperationProgress{
					Type:    "node_speedtest",
					Status:  "failed",
					Stage:   "panic",
					Error:   message,
					Message: "节点测速异常",
				})
			}
		}()
		lastPercent := -1
		lastCompletedStep := 0
		for p := range wrappedChan {
			if p.NodeID > 0 && !p.Testing {
				s.updateNodeLatency(p.NodeID, p.Latency)
			}

			status := "running"
			stage := "node_testing"
			message := "节点测速中"
			if !p.Testing {
				stage = "node_done"
				message = "节点测速完成"
			}

			if !p.Testing && s.shouldLogBatchProgress(p, &lastPercent, &lastCompletedStep) {
				s.updateSpeedTestOperation(op, "节点测速进行中", speedTestProgressDetail(p))
			}

			if !p.Testing && p.Total > 0 && p.Completed == p.Total {
				status = "success"
				stage = "finished"
				message = "节点测速完成"
				if p.Success == 0 && p.Failed > 0 {
					status = "failed"
					message = "节点测速失败"
					s.failSpeedTestOperation(op, "节点测速失败", speedTestProgressDetail(p))
				} else {
					s.finishSpeedTestOperation(op, "节点测速完成", speedTestProgressDetail(p))
				}
			}

			s.publishSpeedTestProgress(job, dto.OperationProgress{
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
			})
		}
	}()
	return wrappedChan
}

func (s *NodeService) publishSpeedTestProgress(job *nodeSpeedTestJob, progress dto.OperationProgress) {
	progressCopy := progress
	s.speedMu.Lock()
	if s.speedJob != job {
		s.speedMu.Unlock()
		return
	}

	if len(progress.Nodes) > 0 {
		job.targetNodes = append([]dto.NodeInfo(nil), progress.Nodes...)
	}
	if len(progressCopy.Nodes) == 0 && len(job.targetNodes) > 0 && progress.Stage == "start" {
		progressCopy.Nodes = append([]dto.NodeInfo(nil), job.targetNodes...)
	}

	job.lastProgress = cloneOperationProgress(&progressCopy)
	terminal := progress.Status != "running"
	if terminal {
		job.running = false
		job.finishedAt = time.Now()
	}

	subscribers := make([]chan dto.OperationProgress, 0, len(job.subscribers))
	for ch := range job.subscribers {
		subscribers = append(subscribers, ch)
	}
	if terminal {
		job.subscribers = make(map[chan dto.OperationProgress]struct{})
	}
	s.speedMu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- progressCopy:
		default:
		}
		if terminal {
			close(ch)
		}
	}
}

func cloneOperationProgress(progress *dto.OperationProgress) *dto.OperationProgress {
	if progress == nil {
		return nil
	}
	cloned := *progress
	if progress.Nodes != nil {
		cloned.Nodes = append([]dto.NodeInfo(nil), progress.Nodes...)
	}
	return &cloned
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

func ToNodeInfos(nodes []*model.Node) []dto.NodeInfo {
	infos := make([]dto.NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		infos = append(infos, dto.NodeInfo{
			ID:             node.ID,
			SubscriptionID: node.SubscriptionID,
			Name:           node.Name,
			Protocol:       node.Protocol,
			ProtocolLabel:  types.ProtocolLabel(node.Protocol),
			Address:        node.Address,
			Port:           node.Port,
			RawURL:         node.RawURL,
			OutboundConfig: node.OutboundConfig,
			Latency:        node.Latency,
			CreatedAt:      node.CreatedAt,
			UpdatedAt:      node.UpdatedAt,
		})
	}
	return infos
}

func (s *NodeService) startSpeedTestOperation(message string, detail map[string]any) *OperationLog {
	if s.logger == nil {
		return nil
	}
	op, opErr := s.logger.StartOperation(constants.TagSpeedtest, message, detail)
	if opErr != nil {
		log.Printf("NodeService: 启动操作日志失败 message=%s err=%v", message, opErr)
	}
	return op
}

func (s *NodeService) updateSpeedTestOperation(op *OperationLog, message string, detail map[string]any) {
	if op != nil {
		_ = op.Update(message, detail)
	}
}

func (s *NodeService) finishSpeedTestOperation(op *OperationLog, message string, detail map[string]any) {
	if op != nil {
		_ = op.Success(message, detail)
	}
}

func (s *NodeService) failSpeedTestOperation(op *OperationLog, message string, detail map[string]any) {
	if op != nil {
		_ = op.Fail(message, detail)
	}
}

func (s *NodeService) SetActive(id uint) error {
	node, err := s.Get(id)
	if err != nil {
		return err
	}
	return s.xraySvc.SetActiveNode(id, node.OutboundConfig)
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

type NodeSummary = dto.NodeSummaryDTO

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

func (s *NodeService) GetProtocols() []dto.ProtocolOption {
	options := make([]dto.ProtocolOption, len(constants.NodeProtocols))
	for i, p := range constants.NodeProtocols {
		options[i] = dto.ProtocolOption{Value: p, Label: types.ProtocolLabel(p)}
	}
	return options
}

func toSpeedNodes(nodes []*model.Node) []speedtest.Node {
	speedNodes := make([]speedtest.Node, len(nodes))
	for i, n := range nodes {
		speedNodes[i] = speedtest.Node{
			ID:             n.ID,
			OutboundConfig: n.OutboundConfig,
		}
	}
	return speedNodes
}

func (s *NodeService) updateNodeLatency(nodeID uint, latency int64) {
	if err := s.repo.UpdateLatency(nodeID, latency); err != nil {
		if s.logger != nil {
			s.logger.Error(constants.TagSpeedtest, "更新节点延迟失败", map[string]any{"node_id": nodeID, "error": err.Error()})
		}
	}
}

func (s *NodeService) listWithPinnedActiveNode(filter model.NodeFilter) ([]*model.Node, string, bool, error) {
	activeID := s.cfg.GetActiveNodeID()
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
	if node == nil {
		return false
	}
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
		if nodeMatchesLatencyStatus(node, status) {
			return true
		}
	}
	return false
}

func nodeMatchesLatencyStatus(node *model.Node, status string) bool {
	switch status {
	case constants.LatencyStatusPending:
		return node.Latency == constants.LatencyUntested
	case constants.LatencyStatusAvailable:
		return node.Latency >= constants.LatencyMinValid
	case constants.LatencyStatusTimeout:
		return node.Latency == constants.LatencyTimeout
	default:
		return false
	}
}

func nodeMatchesKeyword(node *model.Node, keyword string) bool {
	if node == nil || keyword == "" {
		return true
	}
	if containsCaseInsensitive(node.Name, keyword) ||
		containsCaseInsensitive(node.Address, keyword) ||
		containsCaseInsensitive(node.Protocol, keyword) ||
		containsCaseInsensitive(node.RawURL, keyword) {
		return true
	}
	return strconv.Itoa(node.Port) == keyword || containsCaseInsensitive(strconv.Itoa(node.Port), keyword)
}

func containsCaseInsensitive(text, keyword string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

func (s *NodeService) shouldLogBatchProgress(p speedtest.Progress, lastPercent *int, lastCompletedStep *int) bool {
	if p.Total <= 0 || p.Completed <= 0 {
		return false
	}
	if p.Completed == p.Total {
		return false
	}
	if p.Completed == 1 {
		*lastCompletedStep = 1
		*lastPercent = max(*lastPercent, 0)
		return true
	}
	if p.Completed >= *lastCompletedStep+batchProgressStepNode {
		*lastCompletedStep = p.Completed
		return true
	}
	percent := p.Completed * 100 / p.Total
	if percent >= *lastPercent+batchProgressStepPercent {
		*lastPercent = percent
		return true
	}
	return false
}
