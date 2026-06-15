package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	actionspec "github.com/go-tangra/go-tangra-actions/workflow"

	"github.com/go-tangra/go-tangra-executor/internal/data"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// ActionService implements the ExecutorActionService gRPC service: the admin
// repository of go-tangra-actions action packages.
type ActionService struct {
	executorV1.UnimplementedExecutorActionServiceServer

	log          *log.Helper
	actionRepo   *data.ActionRepo
	portalClient *data.PortalClient
}

// NewActionService creates a new ActionService.
func NewActionService(
	ctx *bootstrap.Context,
	actionRepo *data.ActionRepo,
	portalClient *data.PortalClient,
) *ActionService {
	return &ActionService{
		log:          ctx.NewLoggerHelper("executor/service/action"),
		actionRepo:   actionRepo,
		portalClient: portalClient,
	}
}

// CreateAction validates the manifest and stores a new action package.
func (s *ActionService) CreateAction(ctx context.Context, req *executorV1.CreateActionRequest) (*executorV1.CreateActionResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	def, err := actionspec.ParseAction([]byte(req.Manifest))
	if err != nil {
		return nil, executorV1.ErrorBadRequest("invalid action manifest: %v", err)
	}

	files := protoFilesToInput(req.Files)
	contentHash := computeActionHash(req.Manifest, files)

	entity, err := s.actionRepo.Create(ctx, tenantID, req.Name, req.Description, def.Runs.Using, req.Manifest, contentHash, req.Enabled, createdBy, files)
	if err != nil {
		return nil, err
	}

	return &executorV1.CreateActionResponse{Action: s.actionRepo.ToProto(entity)}, nil
}

// GetAction retrieves an action by ID.
func (s *ActionService) GetAction(ctx context.Context, req *executorV1.GetActionRequest) (*executorV1.GetActionResponse, error) {
	entity, err := s.actionRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorNotFound("action not found")
	}
	return &executorV1.GetActionResponse{Action: s.actionRepo.ToProto(entity)}, nil
}

// ListActions lists actions with pagination and filters.
func (s *ActionService) ListActions(ctx context.Context, req *executorV1.ListActionsRequest) (*executorV1.ListActionsResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	var page, pageSize uint32
	if req.Page != nil {
		page = *req.Page
	}
	if req.PageSize != nil {
		pageSize = *req.PageSize
	}

	entities, total, err := s.actionRepo.ListByTenant(ctx, tenantID, req.Name, req.Using, req.Enabled, page, pageSize)
	if err != nil {
		return nil, err
	}

	actions := make([]*executorV1.Action, 0, len(entities))
	for _, e := range entities {
		actions = append(actions, s.actionRepo.ToProto(e))
	}

	return &executorV1.ListActionsResponse{Actions: actions, Total: uint32(total)}, nil
}

// UpdateAction updates an action; a manifest change requires password verification.
func (s *ActionService) UpdateAction(ctx context.Context, req *executorV1.UpdateActionRequest) (*executorV1.UpdateActionResponse, error) {
	updatedBy := getUserIDAsUint32(ctx)

	entity, err := s.actionRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorNotFound("action not found")
	}

	var newUsing, newHash *string
	var newVersion *int
	var replaceFiles bool
	var files []data.ActionFileInput

	// The manifest is sent only when the package content (manifest and/or files)
	// changed; its presence marks a content update that replaces the file set.
	if req.Manifest != nil {
		if req.Password == nil || *req.Password == "" {
			return nil, executorV1.ErrorPasswordRequired("password is required when updating action content")
		}
		username := getUsernameFromContext(ctx)
		if username == "" {
			return nil, executorV1.ErrorUnauthorized("cannot determine username for password verification")
		}
		verified, verifyErr := s.portalClient.VerifyCredential(ctx, username, *req.Password)
		if verifyErr != nil {
			s.log.Errorf("Password verification failed: %v", verifyErr)
			return nil, executorV1.ErrorInternalServerError("password verification service unavailable")
		}
		if !verified {
			return nil, executorV1.ErrorPasswordVerificationFailed("password verification failed")
		}

		def, parseErr := actionspec.ParseAction([]byte(*req.Manifest))
		if parseErr != nil {
			return nil, executorV1.ErrorBadRequest("invalid action manifest: %v", parseErr)
		}

		files = protoFilesToInput(req.Files)
		using := def.Runs.Using
		hash := computeActionHash(*req.Manifest, files)
		v := entity.Version + 1
		newUsing = &using
		newHash = &hash
		newVersion = &v
		replaceFiles = true
	}

	updated, err := s.actionRepo.Update(ctx, req.Id, req.Description, newUsing, req.Manifest, newHash, req.Enabled, newVersion, updatedBy, replaceFiles, files)
	if err != nil {
		return nil, err
	}

	return &executorV1.UpdateActionResponse{Action: s.actionRepo.ToProto(updated)}, nil
}

// DeleteAction deletes an action (its files cascade).
func (s *ActionService) DeleteAction(ctx context.Context, req *executorV1.DeleteActionRequest) (*emptypb.Empty, error) {
	entity, err := s.actionRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorNotFound("action not found")
	}
	if err := s.actionRepo.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// protoFilesToInput converts proto ActionFiles to repo inputs.
func protoFilesToInput(files []*executorV1.ActionFile) []data.ActionFileInput {
	out := make([]data.ActionFileInput, 0, len(files))
	for _, f := range files {
		if f.GetPath() == "" {
			continue
		}
		out = append(out, data.ActionFileInput{Path: f.GetPath(), Content: f.GetContent()})
	}
	return out
}

// computeActionHash hashes the manifest plus the package files (path-sorted) so
// any content change produces a new digest.
func computeActionHash(manifest string, files []data.ActionFileInput) string {
	sorted := make([]data.ActionFileInput, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	h := sha256.New()
	h.Write([]byte(manifest))
	for _, f := range sorted {
		h.Write([]byte{0})
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.Content))
	}
	return hex.EncodeToString(h.Sum(nil))
}
