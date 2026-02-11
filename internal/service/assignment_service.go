package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-tangra/go-tangra-executor/internal/data"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// AssignmentService implements the ExecutorAssignmentService gRPC service
type AssignmentService struct {
	executorV1.UnimplementedExecutorAssignmentServiceServer

	log        *log.Helper
	assignRepo *data.AssignmentRepo
	scriptRepo *data.ScriptRepo
}

// NewAssignmentService creates a new AssignmentService
func NewAssignmentService(
	ctx *bootstrap.Context,
	assignRepo *data.AssignmentRepo,
	scriptRepo *data.ScriptRepo,
) *AssignmentService {
	return &AssignmentService{
		log:        ctx.NewLoggerHelper("executor/service/assignment"),
		assignRepo: assignRepo,
		scriptRepo: scriptRepo,
	}
}

// AssignScript assigns a script to a client
func (s *AssignmentService) AssignScript(ctx context.Context, req *executorV1.AssignScriptRequest) (*executorV1.AssignScriptResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	// Verify script exists
	script, err := s.scriptRepo.GetByID(ctx, req.ScriptId)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, executorV1.ErrorScriptNotFound("script not found")
	}

	// Check for duplicate
	exists, err := s.assignRepo.Exists(ctx, tenantID, req.ScriptId, req.ClientId)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, executorV1.ErrorAssignmentAlreadyExists("script is already assigned to this client")
	}

	entity, err := s.assignRepo.Create(ctx, tenantID, req.ScriptId, req.ClientId, createdBy)
	if err != nil {
		return nil, err
	}

	return &executorV1.AssignScriptResponse{
		Assignment: s.assignRepo.ToProto(entity),
	}, nil
}

// UnassignScript removes a script assignment from a client
func (s *AssignmentService) UnassignScript(ctx context.Context, req *executorV1.UnassignScriptRequest) (*emptypb.Empty, error) {
	tenantID := getTenantIDFromContext(ctx)

	if err := s.assignRepo.Delete(ctx, tenantID, req.ScriptId, req.ClientId); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ListAssignments lists assignments for a script
func (s *AssignmentService) ListAssignments(ctx context.Context, req *executorV1.ListAssignmentsRequest) (*executorV1.ListAssignmentsResponse, error) {
	entities, err := s.assignRepo.ListByScriptID(ctx, req.ScriptId)
	if err != nil {
		return nil, err
	}

	assignments := make([]*executorV1.ScriptAssignment, 0, len(entities))
	for _, e := range entities {
		assignments = append(assignments, s.assignRepo.ToProto(e))
	}

	return &executorV1.ListAssignmentsResponse{
		Assignments: assignments,
	}, nil
}

// ListClientScripts lists scripts assigned to a client
func (s *AssignmentService) ListClientScripts(ctx context.Context, req *executorV1.ListClientScriptsRequest) (*executorV1.ListClientScriptsResponse, error) {
	entities, err := s.assignRepo.ListByClientID(ctx, req.ClientId)
	if err != nil {
		return nil, err
	}

	assignments := make([]*executorV1.ScriptAssignment, 0, len(entities))
	for _, e := range entities {
		proto := s.assignRepo.ToProto(e)
		// Optionally populate script details
		script, sErr := s.scriptRepo.GetByID(ctx, e.ScriptID)
		if sErr == nil && script != nil {
			proto.Script = s.scriptRepo.ToProto(script)
		}
		assignments = append(assignments, proto)
	}

	return &executorV1.ListClientScriptsResponse{
		Assignments: assignments,
	}, nil
}
