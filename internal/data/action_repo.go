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
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/action"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/actionfile"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// ActionFileInput is a package file to persist alongside an action.
type ActionFileInput struct {
	Path    string
	Content string
}

// ActionRepo handles database operations for the action repository.
type ActionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewActionRepo creates a new ActionRepo.
func NewActionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ActionRepo {
	return &ActionRepo{
		log:       ctx.NewLoggerHelper("executor/repo/action"),
		entClient: entClient,
	}
}

// Create inserts a new action and its package files in a single transaction.
func (r *ActionRepo) Create(ctx context.Context, tenantID uint32, name, description, using, manifest, contentHash string, enabled bool, createdBy *uint32, files []ActionFileInput) (*ent.Action, error) {
	tx, err := r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("begin tx failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create action failed")
	}

	id := uuid.New().String()
	builder := tx.Action.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetName(name).
		SetUsing(using).
		SetManifest(manifest).
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
		_ = tx.Rollback()
		if ent.IsConstraintError(err) {
			return nil, executorV1.ErrorInternalServerError("action '%s' already exists", name)
		}
		r.log.Errorf("create action failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create action failed")
	}

	if err := createActionFiles(ctx, tx, id, files); err != nil {
		_ = tx.Rollback()
		r.log.Errorf("create action files failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create action files failed")
	}

	if err := tx.Commit(); err != nil {
		r.log.Errorf("commit tx failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("create action failed")
	}

	return r.GetByID(ctx, entity.ID)
}

func createActionFiles(ctx context.Context, tx *ent.Tx, actionID string, files []ActionFileInput) error {
	for _, f := range files {
		if f.Path == "" {
			continue
		}
		if _, err := tx.ActionFile.Create().
			SetID(uuid.New().String()).
			SetActionID(actionID).
			SetPath(f.Path).
			SetContent(f.Content).
			SetCreateTime(time.Now()).
			Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

// GetByID retrieves an action (with its files) by ID.
func (r *ActionRepo) GetByID(ctx context.Context, id string) (*ent.Action, error) {
	entity, err := r.entClient.Client().Action.Query().
		Where(action.IDEQ(id)).
		WithFiles().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get action failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("get action failed")
	}
	return entity, nil
}

// GetByName retrieves an enabled action (with its files) by name within a tenant.
// Used to resolve a workflow `uses:` reference for execution on a host.
func (r *ActionRepo) GetByName(ctx context.Context, tenantID uint32, name string) (*ent.Action, error) {
	entity, err := r.entClient.Client().Action.Query().
		Where(
			action.TenantIDEQ(tenantID),
			action.NameEQ(name),
		).
		WithFiles().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get action by name failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("get action failed")
	}
	return entity, nil
}

// ListByTenant lists actions for a tenant with pagination and filters.
func (r *ActionRepo) ListByTenant(ctx context.Context, tenantID uint32, name, using *string, enabled *bool, page, pageSize uint32) ([]*ent.Action, int, error) {
	query := r.entClient.Client().Action.Query().
		Where(action.TenantIDEQ(tenantID))

	if name != nil && *name != "" {
		query = query.Where(action.NameContains(*name))
	}
	if using != nil && *using != "" {
		query = query.Where(action.UsingEQ(*using))
	}
	if enabled != nil {
		query = query.Where(action.EnabledEQ(*enabled))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count actions failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("count actions failed")
	}

	if page > 0 && pageSize > 0 {
		offset := int((page - 1) * pageSize)
		query = query.Offset(offset).Limit(int(pageSize))
	}

	entities, err := query.
		Order(ent.Desc(action.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list actions failed: %s", err.Error())
		return nil, 0, executorV1.ErrorInternalServerError("list actions failed")
	}

	return entities, total, nil
}

// Update updates an action's metadata. When replaceFiles is true the package
// files are replaced with the provided set (within a transaction).
func (r *ActionRepo) Update(ctx context.Context, id string, description, using, manifest, contentHash *string, enabled *bool, version *int, updatedBy *uint32, replaceFiles bool, files []ActionFileInput) (*ent.Action, error) {
	tx, err := r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("begin tx failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("update action failed")
	}

	builder := tx.Action.UpdateOneID(id).SetUpdateTime(time.Now())
	if description != nil {
		builder.SetDescription(*description)
	}
	if using != nil {
		builder.SetUsing(*using)
	}
	if manifest != nil {
		builder.SetManifest(*manifest)
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

	if _, err := builder.Save(ctx); err != nil {
		_ = tx.Rollback()
		if ent.IsNotFound(err) {
			return nil, executorV1.ErrorNotFound("action not found")
		}
		r.log.Errorf("update action failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("update action failed")
	}

	if replaceFiles {
		if _, err := tx.ActionFile.Delete().
			Where(actionfile.ActionIDEQ(id)).
			Exec(ctx); err != nil {
			_ = tx.Rollback()
			r.log.Errorf("clear action files failed: %s", err.Error())
			return nil, executorV1.ErrorInternalServerError("update action files failed")
		}
		if err := createActionFiles(ctx, tx, id, files); err != nil {
			_ = tx.Rollback()
			r.log.Errorf("recreate action files failed: %s", err.Error())
			return nil, executorV1.ErrorInternalServerError("update action files failed")
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Errorf("commit tx failed: %s", err.Error())
		return nil, executorV1.ErrorInternalServerError("update action failed")
	}

	return r.GetByID(ctx, id)
}

// Delete deletes an action; its files cascade.
func (r *ActionRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().Action.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return executorV1.ErrorNotFound("action not found")
		}
		r.log.Errorf("delete action failed: %s", err.Error())
		return executorV1.ErrorInternalServerError("delete action failed")
	}
	return nil
}

// ToProto converts an ent.Action (with files eager-loaded) to executorV1.Action.
func (r *ActionRepo) ToProto(entity *ent.Action) *executorV1.Action {
	if entity == nil {
		return nil
	}

	proto := &executorV1.Action{
		Id:          entity.ID,
		TenantId:    derefUint32(entity.TenantID),
		Name:        entity.Name,
		Description: entity.Description,
		Using:       entity.Using,
		Manifest:    entity.Manifest,
		ContentHash: entity.ContentHash,
		Version:     int32(entity.Version),
		Enabled:     entity.Enabled,
		Files:       actionFilesToProto(entity.Edges.Files),
	}

	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreatedAt = timestamppb.New(*entity.CreateTime)
	}
	if entity.UpdateTime != nil && !entity.UpdateTime.IsZero() {
		proto.UpdatedAt = timestamppb.New(*entity.UpdateTime)
	}

	return proto
}

// ActionFilesToProto exposes file mapping for the client-facing resolve path.
func (r *ActionRepo) ActionFilesToProto(files []*ent.ActionFile) []*executorV1.ActionFile {
	return actionFilesToProto(files)
}

func actionFilesToProto(files []*ent.ActionFile) []*executorV1.ActionFile {
	out := make([]*executorV1.ActionFile, 0, len(files))
	for _, f := range files {
		out = append(out, &executorV1.ActionFile{Path: f.Path, Content: f.Content})
	}
	return out
}
