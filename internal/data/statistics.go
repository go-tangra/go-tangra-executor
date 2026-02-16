package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-executor/internal/data/ent"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/executionlog"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/script"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/scriptassignment"
)

// StatisticsRepo provides methods for collecting Executor statistics
type StatisticsRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewStatisticsRepo creates a new StatisticsRepo
func NewStatisticsRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *StatisticsRepo {
	return &StatisticsRepo{
		entClient: entClient,
		log:       ctx.NewLoggerHelper("executor/statistics/repo"),
	}
}

// GetScriptCount returns the total number of scripts for a tenant
func (r *StatisticsRepo) GetScriptCount(ctx context.Context, tenantID uint32) (int64, error) {
	count, err := r.entClient.Client().Script.Query().
		Where(script.TenantIDEQ(tenantID)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

// GetScriptCountByEnabled returns the count of scripts with the given enabled status
func (r *StatisticsRepo) GetScriptCountByEnabled(ctx context.Context, tenantID uint32, enabled bool) (int64, error) {
	count, err := r.entClient.Client().Script.Query().
		Where(
			script.TenantIDEQ(tenantID),
			script.EnabledEQ(enabled),
		).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

// GetAssignmentCount returns the total number of assignments for a tenant
func (r *StatisticsRepo) GetAssignmentCount(ctx context.Context, tenantID uint32) (int64, error) {
	count, err := r.entClient.Client().ScriptAssignment.Query().
		Where(scriptassignment.TenantIDEQ(tenantID)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

// GetExecutionCount returns the total number of executions for a tenant
func (r *StatisticsRepo) GetExecutionCount(ctx context.Context, tenantID uint32) (int64, error) {
	count, err := r.entClient.Client().ExecutionLog.Query().
		Where(executionlog.TenantIDEQ(tenantID)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

// GetExecutionCountByStatus returns the count of executions with the given status
func (r *StatisticsRepo) GetExecutionCountByStatus(ctx context.Context, tenantID uint32, status executionlog.Status) (int64, error) {
	count, err := r.entClient.Client().ExecutionLog.Query().
		Where(
			executionlog.TenantIDEQ(tenantID),
			executionlog.StatusEQ(status),
		).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

// GetExecutionCountSince returns the count of executions since the given time
func (r *StatisticsRepo) GetExecutionCountSince(ctx context.Context, tenantID uint32, since time.Time) (int64, error) {
	count, err := r.entClient.Client().ExecutionLog.Query().
		Where(
			executionlog.TenantIDEQ(tenantID),
			executionlog.CreateTimeGTE(since),
		).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

// GetRecentErrors returns the most recent failed/rejected executions
func (r *StatisticsRepo) GetRecentErrors(ctx context.Context, tenantID uint32, limit int) ([]*ent.ExecutionLog, error) {
	return r.entClient.Client().ExecutionLog.Query().
		Where(
			executionlog.TenantIDEQ(tenantID),
			executionlog.StatusIn(
				executionlog.StatusFAILED,
				executionlog.StatusREJECTED_HASH_MISMATCH,
				executionlog.StatusREJECTED_NOT_APPROVED,
			),
		).
		Order(ent.Desc(executionlog.FieldCreateTime)).
		Limit(limit).
		All(ctx)
}
