package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-tangra/go-tangra-executor/internal/data"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// WorkflowService implements the ExecutorWorkflowService gRPC service: the
// repository of saved, runnable go-tangra-actions workflows.
type WorkflowService struct {
	executorV1.UnimplementedExecutorWorkflowServiceServer

	log          *log.Helper
	workflowRepo *data.WorkflowRepo
}

// NewWorkflowService creates a new WorkflowService.
func NewWorkflowService(ctx *bootstrap.Context, workflowRepo *data.WorkflowRepo) *WorkflowService {
	return &WorkflowService{
		log:          ctx.NewLoggerHelper("executor/service/workflow"),
		workflowRepo: workflowRepo,
	}
}

// CreateWorkflow stores a new workflow.
func (s *WorkflowService) CreateWorkflow(ctx context.Context, req *executorV1.CreateWorkflowRequest) (*executorV1.CreateWorkflowResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	contentHash := ComputeContentHash(req.Content)
	entity, err := s.workflowRepo.Create(ctx, tenantID, req.Name, req.Description, req.Content, contentHash, req.Enabled, createdBy)
	if err != nil {
		return nil, err
	}
	return &executorV1.CreateWorkflowResponse{Workflow: s.workflowRepo.ToProto(entity)}, nil
}

// GetWorkflow retrieves a workflow by ID.
func (s *WorkflowService) GetWorkflow(ctx context.Context, req *executorV1.GetWorkflowRequest) (*executorV1.GetWorkflowResponse, error) {
	entity, err := s.workflowRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorNotFound("workflow not found")
	}
	return &executorV1.GetWorkflowResponse{Workflow: s.workflowRepo.ToProto(entity)}, nil
}

// ListWorkflows lists workflows with pagination and filters.
func (s *WorkflowService) ListWorkflows(ctx context.Context, req *executorV1.ListWorkflowsRequest) (*executorV1.ListWorkflowsResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	var page, pageSize uint32
	if req.Page != nil {
		page = *req.Page
	}
	if req.PageSize != nil {
		pageSize = *req.PageSize
	}

	entities, total, err := s.workflowRepo.ListByTenant(ctx, tenantID, req.Name, req.Enabled, page, pageSize)
	if err != nil {
		return nil, err
	}
	workflows := make([]*executorV1.Workflow, 0, len(entities))
	for _, e := range entities {
		workflows = append(workflows, s.workflowRepo.ToProto(e))
	}
	return &executorV1.ListWorkflowsResponse{Workflows: workflows, Total: uint32(total)}, nil
}

// UpdateWorkflow updates a workflow; a content change rehashes and bumps the version.
func (s *WorkflowService) UpdateWorkflow(ctx context.Context, req *executorV1.UpdateWorkflowRequest) (*executorV1.UpdateWorkflowResponse, error) {
	updatedBy := getUserIDAsUint32(ctx)

	entity, err := s.workflowRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorNotFound("workflow not found")
	}

	var newHash *string
	var newVersion *int
	if req.Content != nil && *req.Content != entity.Content {
		hash := ComputeContentHash(*req.Content)
		newHash = &hash
		v := entity.Version + 1
		newVersion = &v
	}

	updated, err := s.workflowRepo.Update(ctx, req.Id, req.Name, req.Description, req.Content, newHash, req.Enabled, newVersion, updatedBy)
	if err != nil {
		return nil, err
	}
	return &executorV1.UpdateWorkflowResponse{Workflow: s.workflowRepo.ToProto(updated)}, nil
}

// DeleteWorkflow deletes a workflow.
func (s *WorkflowService) DeleteWorkflow(ctx context.Context, req *executorV1.DeleteWorkflowRequest) (*emptypb.Empty, error) {
	entity, err := s.workflowRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorNotFound("workflow not found")
	}
	if err := s.workflowRepo.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
