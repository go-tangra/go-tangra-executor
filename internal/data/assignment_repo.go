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
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/scriptassignment"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// AssignmentRepo handles database operations for script assignments
type AssignmentRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewAssignmentRepo creates a new AssignmentRepo
func NewAssignmentRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AssignmentRepo {
	return &AssignmentRepo{
		log:       ctx.NewLoggerHelper("executor/repo/assignment"),
		entClient: entClient,
	}
}

// Create creates a new script assignment
func (r *AssignmentRepo) Create(ctx context.Context, tenantID uint32, scriptID, clientID string, createdBy *uint32) (*ent.ScriptAssignment, error) {
	id := uuid.New().String()

	builder := r.entClient.Client().ScriptAssignment.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetScriptID(scriptID).
		SetClientID(clientID).
		SetCreateTime(time.Now())

	if createdBy != nil {
		builder.SetCreateBy(*createdBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create assignment failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create assignment failed")
	}

	return entity, nil
}

// Exists checks if an assignment exists
func (r *AssignmentRepo) Exists(ctx context.Context, tenantID uint32, scriptID, clientID string) (bool, error) {
	exists, err := r.entClient.Client().ScriptAssignment.Query().
		Where(
			scriptassignment.TenantIDEQ(tenantID),
			scriptassignment.ScriptIDEQ(scriptID),
			scriptassignment.ClientIDEQ(clientID),
		).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("check assignment exists failed: %s", err.Error())
		return false, executorV1.ErrorInternalServerError("check assignment failed")
	}
	return exists, nil
}

// ExistsAnyTenant checks if an assignment exists for any tenant (used by client service with mTLS)
func (r *AssignmentRepo) ExistsAnyTenant(ctx context.Context, scriptID, clientID string) (bool, error) {
	exists, err := r.entClient.Client().ScriptAssignment.Query().
		Where(
			scriptassignment.ScriptIDEQ(scriptID),
			scriptassignment.ClientIDEQ(clientID),
		).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("check assignment exists failed: %s", err.Error())
		return false, executorV1.ErrorInternalServerError("check assignment failed")
	}
	return exists, nil
}

// Delete deletes an assignment by script_id, client_id, and tenant_id
func (r *AssignmentRepo) Delete(ctx context.Context, tenantID uint32, scriptID, clientID string) error {
	deleted, err := r.entClient.Client().ScriptAssignment.Delete().
		Where(
			scriptassignment.TenantIDEQ(tenantID),
			scriptassignment.ScriptIDEQ(scriptID),
			scriptassignment.ClientIDEQ(clientID),
		).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("delete assignment failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("delete assignment failed")
	}
	if deleted == 0 {
		return executorV1.ErrorAssignmentNotFound("assignment not found")
	}
	return nil
}

// ListByScriptID lists assignments for a script
func (r *AssignmentRepo) ListByScriptID(ctx context.Context, scriptID string) ([]*ent.ScriptAssignment, error) {
	entities, err := r.entClient.Client().ScriptAssignment.Query().
		Where(scriptassignment.ScriptIDEQ(scriptID)).
		Order(ent.Desc(scriptassignment.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list assignments by script failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("list assignments failed")
	}
	return entities, nil
}

// ListByClientID lists assignments for a client
func (r *AssignmentRepo) ListByClientID(ctx context.Context, clientID string) ([]*ent.ScriptAssignment, error) {
	entities, err := r.entClient.Client().ScriptAssignment.Query().
		Where(scriptassignment.ClientIDEQ(clientID)).
		Order(ent.Desc(scriptassignment.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list assignments by client failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("list assignments failed")
	}
	return entities, nil
}

// DeleteByScriptID deletes all assignments for a script (used when deleting a script)
func (r *AssignmentRepo) DeleteByScriptID(ctx context.Context, scriptID string) error {
	_, err := r.entClient.Client().ScriptAssignment.Delete().
		Where(scriptassignment.ScriptIDEQ(scriptID)).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("delete assignments by script failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("delete assignments failed")
	}
	return nil
}

// ToProto converts an ent.ScriptAssignment to executorV1.ScriptAssignment
func (r *AssignmentRepo) ToProto(entity *ent.ScriptAssignment) *executorV1.ScriptAssignment {
	if entity == nil {
		return nil
	}

	proto := &executorV1.ScriptAssignment{
		Id:       entity.ID,
		TenantId: derefUint32(entity.TenantID),
		ScriptId: entity.ScriptID,
		ClientId: entity.ClientID,
	}

	if entity.CreateBy != nil {
		proto.CreatedBy = entity.CreateBy
	}
	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreateTime = timestamppb.New(*entity.CreateTime)
	}

	return proto
}
