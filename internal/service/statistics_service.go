package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
	"github.com/go-tangra/go-tangra-executor/internal/data"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/executionlog"
)

type StatisticsService struct {
	executorV1.UnimplementedExecutorStatisticsServiceServer

	log  *log.Helper
	repo *data.StatisticsRepo
}

func NewStatisticsService(ctx *bootstrap.Context, repo *data.StatisticsRepo) *StatisticsService {
	return &StatisticsService{
		log:  ctx.NewLoggerHelper("executor/service/statistics"),
		repo: repo,
	}
}

// GetStatistics returns comprehensive statistics about the Executor system
func (s *StatisticsService) GetStatistics(ctx context.Context, req *executorV1.GetStatisticsRequest) (*executorV1.GetStatisticsResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	if req.TenantId != nil {
		tenantID = *req.TenantId
	}

	// Scripts
	totalScripts, err := s.repo.GetScriptCount(ctx, tenantID)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get script count: %v", err)
		return nil, err
	}

	enabledScripts, err := s.repo.GetScriptCountByEnabled(ctx, tenantID, true)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get enabled script count: %v", err)
		return nil, err
	}

	disabledScripts, err := s.repo.GetScriptCountByEnabled(ctx, tenantID, false)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get disabled script count: %v", err)
		return nil, err
	}

	// Assignments
	totalAssignments, err := s.repo.GetAssignmentCount(ctx, tenantID)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get assignment count: %v", err)
		return nil, err
	}

	// Executions
	totalExecutions, err := s.repo.GetExecutionCount(ctx, tenantID)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get execution count: %v", err)
		return nil, err
	}

	completedExecutions, err := s.repo.GetExecutionCountByStatus(ctx, tenantID, executionlog.StatusCOMPLETED)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get completed execution count: %v", err)
		return nil, err
	}

	failedExecutions, err := s.repo.GetExecutionCountByStatus(ctx, tenantID, executionlog.StatusFAILED)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get failed execution count: %v", err)
		return nil, err
	}

	pendingExecutions, err := s.repo.GetExecutionCountByStatus(ctx, tenantID, executionlog.StatusPENDING)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get pending execution count: %v", err)
		return nil, err
	}

	runningExecutions, err := s.repo.GetExecutionCountByStatus(ctx, tenantID, executionlog.StatusRUNNING)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get running execution count: %v", err)
		return nil, err
	}

	// Rejected = hash mismatch + not approved
	rejectedHash, err := s.repo.GetExecutionCountByStatus(ctx, tenantID, executionlog.StatusREJECTED_HASH_MISMATCH)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get rejected hash execution count: %v", err)
		return nil, err
	}

	rejectedApproval, err := s.repo.GetExecutionCountByStatus(ctx, tenantID, executionlog.StatusREJECTED_NOT_APPROVED)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get rejected approval execution count: %v", err)
		return nil, err
	}

	rejectedExecutions := rejectedHash + rejectedApproval

	// Success rate
	var successRate float64
	if completedExecutions+failedExecutions > 0 {
		successRate = float64(completedExecutions) / float64(completedExecutions+failedExecutions) * 100
	}

	// Time-based
	now := time.Now()
	executionsLast24h, err := s.repo.GetExecutionCountSince(ctx, tenantID, now.Add(-24*time.Hour))
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get executions last 24h: %v", err)
		return nil, err
	}

	executionsLast7d, err := s.repo.GetExecutionCountSince(ctx, tenantID, now.Add(-7*24*time.Hour))
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get executions last 7d: %v", err)
		return nil, err
	}

	// Recent errors
	recentErrorLogs, err := s.repo.GetRecentErrors(ctx, tenantID, 10)
	if err != nil {
		s.log.WithContext(ctx).Errorf("failed to get recent errors: %v", err)
		return nil, err
	}

	recentErrors := make([]*executorV1.RecentError, 0, len(recentErrorLogs))
	for _, e := range recentErrorLogs {
		re := &executorV1.RecentError{
			ExecutionId: e.ID,
			ScriptName:  e.ScriptName,
			ClientId:    e.ClientID,
			ErrorOutput: e.ErrorOutput,
			Status:      string(e.Status),
		}
		if e.CreateTime != nil && !e.CreateTime.IsZero() {
			re.CreatedAt = timestamppb.New(*e.CreateTime)
		}
		recentErrors = append(recentErrors, re)
	}

	return &executorV1.GetStatisticsResponse{
		TotalScripts:        totalScripts,
		EnabledScripts:      enabledScripts,
		DisabledScripts:     disabledScripts,
		TotalAssignments:    totalAssignments,
		TotalExecutions:     totalExecutions,
		CompletedExecutions: completedExecutions,
		FailedExecutions:    failedExecutions,
		PendingExecutions:   pendingExecutions,
		RunningExecutions:   runningExecutions,
		RejectedExecutions:  rejectedExecutions,
		SuccessRate:         successRate,
		ExecutionsLast_24H:  executionsLast24h,
		ExecutionsLast_7D:   executionsLast7d,
		RecentErrors:        recentErrors,
	}, nil
}
