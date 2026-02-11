package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-executor/internal/data/ent"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/executionlog"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// ExecutionLogRepo handles database operations for execution logs
type ExecutionLogRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewExecutionLogRepo creates a new ExecutionLogRepo
func NewExecutionLogRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ExecutionLogRepo {
	return &ExecutionLogRepo{
		log:       ctx.NewLoggerHelper("executor/repo/execution_log"),
		entClient: entClient,
	}
}

// Create creates a new execution log entry
func (r *ExecutionLogRepo) Create(ctx context.Context, tenantID uint32, scriptID, scriptName, clientID, scriptHash, triggerType, status string, createdBy *uint32) (*ent.ExecutionLog, error) {
	id := uuid.New().String()

	builder := r.entClient.Client().ExecutionLog.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetScriptID(scriptID).
		SetScriptName(scriptName).
		SetClientID(clientID).
		SetScriptHash(scriptHash).
		SetTriggerType(executionlog.TriggerType(triggerType)).
		SetStatus(executionlog.Status(status)).
		SetCreateTime(time.Now())

	if createdBy != nil {
		builder.SetCreateBy(*createdBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create execution log failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create execution log failed")
	}

	return entity, nil
}

// GetByID retrieves an execution log by ID
func (r *ExecutionLogRepo) GetByID(ctx context.Context, id string) (*ent.ExecutionLog, error) {
	entity, err := r.entClient.Client().ExecutionLog.Query().
		Where(executionlog.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get execution log failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("get execution log failed")
	}
	return entity, nil
}

// UpdateStatus updates the status of an execution log
func (r *ExecutionLogRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.entClient.Client().ExecutionLog.UpdateOneID(id).
		SetStatus(executionlog.Status(status)).
		Save(ctx)
	if err != nil {
		r.log.Errorf("update execution log status failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("update execution log status failed")
	}
	return nil
}

// UpdateRejection updates an execution log with rejection info
func (r *ExecutionLogRepo) UpdateRejection(ctx context.Context, id, status, reason string) error {
	_, err := r.entClient.Client().ExecutionLog.UpdateOneID(id).
		SetStatus(executionlog.Status(status)).
		SetRejectionReason(reason).
		Save(ctx)
	if err != nil {
		r.log.Errorf("update execution log rejection failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("update execution log rejection failed")
	}
	return nil
}

// UpdateResult updates an execution log with the execution result
func (r *ExecutionLogRepo) UpdateResult(ctx context.Context, id string, exitCode int, output, errorOutput string, durationMs int64) error {
	now := time.Now()
	status := "COMPLETED"
	if exitCode != 0 {
		status = "FAILED"
	}

	builder := r.entClient.Client().ExecutionLog.UpdateOneID(id).
		SetStatus(executionlog.Status(status)).
		SetExitCode(exitCode).
		SetOutput(output).
		SetErrorOutput(errorOutput).
		SetDurationMs(durationMs).
		SetCompletedAt(now)

	_, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("update execution log result failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("update execution log result failed")
	}
	return nil
}

// SetStartedAt marks an execution as running
func (r *ExecutionLogRepo) SetStartedAt(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.entClient.Client().ExecutionLog.UpdateOneID(id).
		SetStatus(executionlog.StatusRUNNING).
		SetStartedAt(now).
		Save(ctx)
	if err != nil {
		r.log.Errorf("set execution started_at failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("set execution started_at failed")
	}
	return nil
}

// ListByTenant lists execution logs with pagination and filters
func (r *ExecutionLogRepo) ListByTenant(ctx context.Context, tenantID uint32, scriptID, clientID *string, status *string, page, pageSize uint32) ([]*ent.ExecutionLog, int, error) {
	query := r.entClient.Client().ExecutionLog.Query().
		Where(executionlog.TenantIDEQ(tenantID))

	if scriptID != nil && *scriptID != "" {
		query = query.Where(executionlog.ScriptIDEQ(*scriptID))
	}
	if clientID != nil && *clientID != "" {
		query = query.Where(executionlog.ClientIDEQ(*clientID))
	}
	if status != nil && *status != "" {
		query = query.Where(executionlog.StatusEQ(executionlog.Status(*status)))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count execution logs failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("count execution logs failed")
	}

	if page > 0 && pageSize > 0 {
		offset := int((page - 1) * pageSize)
		query = query.Offset(offset).Limit(int(pageSize))
	}

	entities, err := query.
		Order(ent.Desc(executionlog.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list execution logs failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("list execution logs failed")
	}

	return entities, total, nil
}

// ToProto converts an ent.ExecutionLog to executorV1.ExecutionLog
func (r *ExecutionLogRepo) ToProto(entity *ent.ExecutionLog) *executorV1.ExecutionLog {
	if entity == nil {
		return nil
	}

	proto := &executorV1.ExecutionLog{
		Id:         entity.ID,
		TenantId:   derefUint32(entity.TenantID),
		ScriptId:   entity.ScriptID,
		ScriptName: entity.ScriptName,
		ClientId:   entity.ClientID,
		ScriptHash: entity.ScriptHash,
	}

	switch entity.TriggerType {
	case executionlog.TriggerTypeCLIENT_PULL:
		proto.TriggerType = executorV1.TriggerType_TRIGGER_TYPE_CLIENT_PULL
	case executionlog.TriggerTypeUI_PUSH:
		proto.TriggerType = executorV1.TriggerType_TRIGGER_TYPE_UI_PUSH
	}

	switch entity.Status {
	case executionlog.StatusPENDING:
		proto.Status = executorV1.ExecutionStatus_EXECUTION_STATUS_PENDING
	case executionlog.StatusRUNNING:
		proto.Status = executorV1.ExecutionStatus_EXECUTION_STATUS_RUNNING
	case executionlog.StatusCOMPLETED:
		proto.Status = executorV1.ExecutionStatus_EXECUTION_STATUS_COMPLETED
	case executionlog.StatusFAILED:
		proto.Status = executorV1.ExecutionStatus_EXECUTION_STATUS_FAILED
	case executionlog.StatusREJECTED_HASH_MISMATCH:
		proto.Status = executorV1.ExecutionStatus_EXECUTION_STATUS_REJECTED_HASH_MISMATCH
	case executionlog.StatusREJECTED_NOT_APPROVED:
		proto.Status = executorV1.ExecutionStatus_EXECUTION_STATUS_REJECTED_NOT_APPROVED
	case executionlog.StatusCLIENT_OFFLINE:
		proto.Status = executorV1.ExecutionStatus_EXECUTION_STATUS_CLIENT_OFFLINE
	}

	if entity.ExitCode != nil {
		proto.ExitCode = intPtr32(*entity.ExitCode)
	}
	if entity.Output != "" {
		proto.Output = &entity.Output
	}
	if entity.ErrorOutput != "" {
		proto.ErrorOutput = &entity.ErrorOutput
	}
	if entity.RejectionReason != "" {
		proto.RejectionReason = &entity.RejectionReason
	}
	if entity.DurationMs != nil {
		proto.DurationMs = entity.DurationMs
	}

	if entity.CreateBy != nil {
		proto.CreatedBy = entity.CreateBy
	}
	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreateTime = timestamppb.New(*entity.CreateTime)
	}
	if entity.StartedAt != nil && !entity.StartedAt.IsZero() {
		proto.StartedAt = timestamppb.New(*entity.StartedAt)
	}
	if entity.CompletedAt != nil && !entity.CompletedAt.IsZero() {
		proto.CompletedAt = timestamppb.New(*entity.CompletedAt)
	}

	return proto
}

func intPtr32(v int) *int32 {
	i := int32(v)
	return &i
}
