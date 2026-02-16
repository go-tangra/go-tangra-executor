package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-executor/internal/data"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// ExecutionService implements the ExecutorExecutionService gRPC service
type ExecutionService struct {
	executorV1.UnimplementedExecutorExecutionServiceServer

	log        *log.Helper
	scriptRepo *data.ScriptRepo
	assignRepo *data.AssignmentRepo
	execRepo   *data.ExecutionLogRepo
	cmdReg     *CommandRegistry
}

// NewExecutionService creates a new ExecutionService
func NewExecutionService(
	ctx *bootstrap.Context,
	scriptRepo *data.ScriptRepo,
	assignRepo *data.AssignmentRepo,
	execRepo *data.ExecutionLogRepo,
	cmdReg *CommandRegistry,
) *ExecutionService {
	return &ExecutionService{
		log:        ctx.NewLoggerHelper("executor/service/execution"),
		scriptRepo: scriptRepo,
		assignRepo: assignRepo,
		execRepo:   execRepo,
		cmdReg:     cmdReg,
	}
}

// TriggerExecution triggers script execution on a client (UI-push)
func (s *ExecutionService) TriggerExecution(ctx context.Context, req *executorV1.TriggerExecutionRequest) (*executorV1.TriggerExecutionResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	// Verify script exists and is enabled
	script, err := s.scriptRepo.GetByID(ctx, req.ScriptId)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, executorV1.ErrorScriptNotFound("script not found")
	}
	if !script.Enabled {
		return nil, executorV1.ErrorScriptDisabled("script is disabled")
	}

	// Verify assignment exists
	exists, err := s.assignRepo.Exists(ctx, tenantID, req.ScriptId, req.ClientId)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, executorV1.ErrorClientNotAssigned("script is not assigned to this client")
	}

	// Create execution log
	execLog, err := s.execRepo.Create(ctx, tenantID, script.ID, script.Name, req.ClientId, script.ContentHash, "UI_PUSH", "PENDING", createdBy)
	if err != nil {
		return nil, err
	}

	// Send command to client via stream
	commandID := uuid.New().String()
	cmd := &executorV1.ExecutionCommand{
		CommandId:   commandID,
		ExecutionId: execLog.ID,
		ScriptId:    script.ID,
		ScriptName:  script.Name,
		ScriptType:  scriptTypeToProto(string(script.ScriptType)),
		Content:     script.Content,
		ContentHash: script.ContentHash,
	}

	if sendErr := s.cmdReg.Send(req.ClientId, cmd); sendErr != nil {
		// Client not connected — update status
		s.log.Warnf("Client %s not connected: %v", req.ClientId, sendErr)
		if updateErr := s.execRepo.UpdateStatus(ctx, execLog.ID, "CLIENT_OFFLINE"); updateErr != nil {
			s.log.Errorf("failed to update execution %s status to CLIENT_OFFLINE: %v", execLog.ID, updateErr)
		}

		// Re-fetch to get updated status
		if updated, fetchErr := s.execRepo.GetByID(ctx, execLog.ID); fetchErr != nil {
			s.log.Errorf("failed to re-fetch execution %s: %v", execLog.ID, fetchErr)
		} else {
			execLog = updated
		}
	}

	return &executorV1.TriggerExecutionResponse{
		Execution: s.execRepo.ToProto(execLog),
	}, nil
}

// GetExecution retrieves execution details
func (s *ExecutionService) GetExecution(ctx context.Context, req *executorV1.GetExecutionRequest) (*executorV1.GetExecutionResponse, error) {
	entity, err := s.execRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorExecutionNotFound("execution not found")
	}

	return &executorV1.GetExecutionResponse{
		Execution: s.execRepo.ToProto(entity),
	}, nil
}

// ListExecutions lists executions with pagination and filters
func (s *ExecutionService) ListExecutions(ctx context.Context, req *executorV1.ListExecutionsRequest) (*executorV1.ListExecutionsResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	var page, pageSize uint32
	if req.Page != nil {
		page = *req.Page
	}
	if req.PageSize != nil {
		pageSize = *req.PageSize
	}

	var statusStr *string
	if req.Status != nil && *req.Status != executorV1.ExecutionStatus_EXECUTION_STATUS_UNSPECIFIED {
		s := executionStatusToString(*req.Status)
		statusStr = &s
	}

	entities, total, err := s.execRepo.ListByTenant(ctx, tenantID, req.ScriptId, req.ClientId, statusStr, page, pageSize)
	if err != nil {
		return nil, err
	}

	executions := make([]*executorV1.ExecutionLog, 0, len(entities))
	for _, e := range entities {
		executions = append(executions, s.execRepo.ToProto(e))
	}

	return &executorV1.ListExecutionsResponse{
		Executions: executions,
		Total:      uint32(total),
	}, nil
}

// GetExecutionOutput retrieves full stdout/stderr for an execution
func (s *ExecutionService) GetExecutionOutput(ctx context.Context, req *executorV1.GetExecutionOutputRequest) (*executorV1.GetExecutionOutputResponse, error) {
	entity, err := s.execRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorExecutionNotFound("execution not found")
	}

	resp := &executorV1.GetExecutionOutputResponse{
		Output:      entity.Output,
		ErrorOutput: entity.ErrorOutput,
	}
	if entity.ExitCode != nil {
		code := int32(*entity.ExitCode)
		resp.ExitCode = &code
	}

	return resp, nil
}

func scriptTypeToProto(t string) executorV1.ScriptType {
	switch t {
	case "BASH":
		return executorV1.ScriptType_SCRIPT_TYPE_BASH
	case "JAVASCRIPT":
		return executorV1.ScriptType_SCRIPT_TYPE_JAVASCRIPT
	case "LUA":
		return executorV1.ScriptType_SCRIPT_TYPE_LUA
	default:
		return executorV1.ScriptType_SCRIPT_TYPE_UNSPECIFIED
	}
}

func executionStatusToString(s executorV1.ExecutionStatus) string {
	switch s {
	case executorV1.ExecutionStatus_EXECUTION_STATUS_PENDING:
		return "PENDING"
	case executorV1.ExecutionStatus_EXECUTION_STATUS_RUNNING:
		return "RUNNING"
	case executorV1.ExecutionStatus_EXECUTION_STATUS_COMPLETED:
		return "COMPLETED"
	case executorV1.ExecutionStatus_EXECUTION_STATUS_FAILED:
		return "FAILED"
	case executorV1.ExecutionStatus_EXECUTION_STATUS_REJECTED_HASH_MISMATCH:
		return "REJECTED_HASH_MISMATCH"
	case executorV1.ExecutionStatus_EXECUTION_STATUS_REJECTED_NOT_APPROVED:
		return "REJECTED_NOT_APPROVED"
	case executorV1.ExecutionStatus_EXECUTION_STATUS_CLIENT_OFFLINE:
		return "CLIENT_OFFLINE"
	default:
		return ""
	}
}
