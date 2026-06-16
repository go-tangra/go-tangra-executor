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
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/workflow"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// WorkflowRepo handles database operations for saved workflows.
type WorkflowRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewWorkflowRepo creates a new WorkflowRepo.
func NewWorkflowRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *WorkflowRepo {
	return &WorkflowRepo{
		log:       ctx.NewLoggerHelper("executor/repo/workflow"),
		entClient: entClient,
	}
}

// Create inserts a new workflow.
func (r *WorkflowRepo) Create(ctx context.Context, tenantID uint32, name, description, content, contentHash string, enabled bool, createdBy *uint32) (*ent.Workflow, error) {
	builder := r.entClient.Client().Workflow.Create().
		SetID(uuid.New().String()).
		SetTenantID(tenantID).
		SetName(name).
		SetContent(content).
		SetContentHash(contentHash).
		SetVersion(1).
		SetEnabled(enabled).
		SetCreateTime(time.Now())
	if description != "" {
		builder.SetDescription(description)
	}
	if createdBy != nil {
		builder.SetCreateBy(*createdBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, executorV1.ErrorInternalServerError("workflow '%s' already exists", name)
		}
		r.log.Errorf("create workflow failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create workflow failed")
	}
	return entity, nil
}

// GetByID retrieves a workflow by ID.
func (r *WorkflowRepo) GetByID(ctx context.Context, id string) (*ent.Workflow, error) {
	entity, err := r.entClient.Client().Workflow.Query().
		Where(workflow.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get workflow failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("get workflow failed")
	}
	return entity, nil
}

// ListByTenant lists workflows for a tenant with pagination and filters.
func (r *WorkflowRepo) ListByTenant(ctx context.Context, tenantID uint32, name *string, enabled *bool, page, pageSize uint32) ([]*ent.Workflow, int, error) {
	query := r.entClient.Client().Workflow.Query().
		Where(workflow.TenantIDEQ(tenantID))

	if name != nil && *name != "" {
		query = query.Where(workflow.NameContains(*name))
	}
	if enabled != nil {
		query = query.Where(workflow.EnabledEQ(*enabled))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count workflows failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("count workflows failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset(int((page - 1) * pageSize)).Limit(int(pageSize))
	}

	entities, err := query.Order(ent.Desc(workflow.FieldCreateTime)).All(ctx)
	if err != nil {
		r.log.Errorf("list workflows failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("list workflows failed")
	}
	return entities, total, nil
}

// Update updates a workflow.
func (r *WorkflowRepo) Update(ctx context.Context, id string, name, description, content, contentHash *string, enabled *bool, version *int, updatedBy *uint32) (*ent.Workflow, error) {
	builder := r.entClient.Client().Workflow.UpdateOneID(id).SetUpdateTime(time.Now())
	if name != nil {
		builder.SetName(*name)
	}
	if description != nil {
		builder.SetDescription(*description)
	}
	if content != nil {
		builder.SetContent(*content)
	}
	if contentHash != nil {
		builder.SetContentHash(*contentHash)
	}
	if enabled != nil {
		builder.SetEnabled(*enabled)
	}
	if version != nil {
		builder.SetVersion(*version)
	}
	if updatedBy != nil {
		builder.SetUpdateBy(*updatedBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, executorV1.ErrorNotFound("workflow not found")
		}
		r.log.Errorf("update workflow failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("update workflow failed")
	}
	return entity, nil
}

// Delete deletes a workflow.
func (r *WorkflowRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().Workflow.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return executorV1.ErrorNotFound("workflow not found")
		}
		r.log.Errorf("delete workflow failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("delete workflow failed")
	}
	return nil
}

// ToProto converts an ent.Workflow to executorV1.Workflow.
func (r *WorkflowRepo) ToProto(entity *ent.Workflow) *executorV1.Workflow {
	if entity == nil {
		return nil
	}
	proto := &executorV1.Workflow{
		Id:          entity.ID,
		TenantId:    derefUint32(entity.TenantID),
		Name:        entity.Name,
		Description: entity.Description,
		Content:     entity.Content,
		ContentHash: entity.ContentHash,
		Version:     int32(entity.Version),
		Enabled:     entity.Enabled,
	}
	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreatedAt = timestamppb.New(*entity.CreateTime)
	}
	if entity.UpdateTime != nil && !entity.UpdateTime.IsZero() {
		proto.UpdatedAt = timestamppb.New(*entity.UpdateTime)
	}
	return proto
}
