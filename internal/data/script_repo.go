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
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/script"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// ScriptRepo handles database operations for scripts
type ScriptRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewScriptRepo creates a new ScriptRepo
func NewScriptRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ScriptRepo {
	return &ScriptRepo{
		log:       ctx.NewLoggerHelper("executor/repo/script"),
		entClient: entClient,
	}
}

// Create creates a new script
func (r *ScriptRepo) Create(ctx context.Context, tenantID uint32, name, description, scriptType, content, contentHash string, enabled bool, createdBy *uint32) (*ent.Script, error) {
	id := uuid.New().String()

	builder := r.entClient.Client().Script.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetName(name).
		SetScriptType(script.ScriptType(scriptType)).
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
		r.log.Errorf("create script failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create script failed")
	}

	return entity, nil
}

// GetByID retrieves a script by ID
func (r *ScriptRepo) GetByID(ctx context.Context, id string) (*ent.Script, error) {
	entity, err := r.entClient.Client().Script.Query().
		Where(script.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get script failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("get script failed")
	}
	return entity, nil
}

// ListByTenant lists scripts for a tenant with pagination and filters
func (r *ScriptRepo) ListByTenant(ctx context.Context, tenantID uint32, scriptType *string, name *string, enabled *bool, page, pageSize uint32) ([]*ent.Script, int, error) {
	query := r.entClient.Client().Script.Query().
		Where(script.TenantIDEQ(tenantID))

	if scriptType != nil && *scriptType != "" {
		query = query.Where(script.ScriptTypeEQ(script.ScriptType(*scriptType)))
	}
	if name != nil && *name != "" {
		query = query.Where(script.NameContains(*name))
	}
	if enabled != nil {
		query = query.Where(script.EnabledEQ(*enabled))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count scripts failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("count scripts failed")
	}

	if page > 0 && pageSize > 0 {
		offset := int((page - 1) * pageSize)
		query = query.Offset(offset).Limit(int(pageSize))
	}

	entities, err := query.
		Order(ent.Desc(script.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list scripts failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("list scripts failed")
	}

	return entities, total, nil
}

// Update updates a script
func (r *ScriptRepo) Update(ctx context.Context, id string, name, description, content, contentHash *string, enabled *bool, version *int, updatedBy *uint32) (*ent.Script, error) {
	builder := r.entClient.Client().Script.UpdateOneID(id).
		SetUpdateTime(time.Now())

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
			return nil, executorV1.ErrorScriptNotFound("script not found")
		}
		r.log.Errorf("update script failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("update script failed")
	}

	return entity, nil
}

// Delete deletes a script
func (r *ScriptRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().Script.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return executorV1.ErrorScriptNotFound("script not found")
		}
		r.log.Errorf("delete script failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("delete script failed")
	}
	return nil
}

// ToProto converts an ent.Script to executorV1.Script
func (r *ScriptRepo) ToProto(entity *ent.Script) *executorV1.Script {
	if entity == nil {
		return nil
	}

	proto := &executorV1.Script{
		Id:          entity.ID,
		TenantId:    derefUint32(entity.TenantID),
		Name:        entity.Name,
		Description: entity.Description,
		Content:     entity.Content,
		ContentHash: entity.ContentHash,
		Version:     int32(entity.Version),
		Enabled:     entity.Enabled,
	}

	switch entity.ScriptType {
	case script.ScriptTypeBASH:
		proto.ScriptType = executorV1.ScriptType_SCRIPT_TYPE_BASH
	case script.ScriptTypeJAVASCRIPT:
		proto.ScriptType = executorV1.ScriptType_SCRIPT_TYPE_JAVASCRIPT
	case script.ScriptTypeLUA:
		proto.ScriptType = executorV1.ScriptType_SCRIPT_TYPE_LUA
	}

	if entity.CreateBy != nil {
		proto.CreatedBy = entity.CreateBy
	}
	if entity.UpdateBy != nil {
		proto.UpdatedBy = entity.UpdateBy
	}
	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreateTime = timestamppb.New(*entity.CreateTime)
	}
	if entity.UpdateTime != nil && !entity.UpdateTime.IsZero() {
		proto.UpdateTime = timestamppb.New(*entity.UpdateTime)
	}

	return proto
}

// derefUint32 safely dereferences a *uint32 pointer
func derefUint32(v *uint32) uint32 {
	if v == nil {
		return 0
	}
	return *v
}
