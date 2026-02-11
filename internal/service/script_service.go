package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-tangra/go-tangra-executor/internal/data"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// ScriptService implements the ExecutorScriptService gRPC service
type ScriptService struct {
	executorV1.UnimplementedExecutorScriptServiceServer

	log          *log.Helper
	scriptRepo   *data.ScriptRepo
	assignRepo   *data.AssignmentRepo
	portalClient *data.PortalClient
}

// NewScriptService creates a new ScriptService
func NewScriptService(
	ctx *bootstrap.Context,
	scriptRepo *data.ScriptRepo,
	assignRepo *data.AssignmentRepo,
	portalClient *data.PortalClient,
) *ScriptService {
	return &ScriptService{
		log:          ctx.NewLoggerHelper("executor/service/script"),
		scriptRepo:   scriptRepo,
		assignRepo:   assignRepo,
		portalClient: portalClient,
	}
}

// CreateScript creates a new script
func (s *ScriptService) CreateScript(ctx context.Context, req *executorV1.CreateScriptRequest) (*executorV1.CreateScriptResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	scriptType := scriptTypeToString(req.ScriptType)
	if scriptType == "" {
		return nil, executorV1.ErrorInvalidScriptType("script type must be BASH, JAVASCRIPT, or LUA")
	}

	contentHash := ComputeContentHash(req.Content)

	entity, err := s.scriptRepo.Create(ctx, tenantID, req.Name, req.Description, scriptType, req.Content, contentHash, req.Enabled, createdBy)
	if err != nil {
		return nil, err
	}

	return &executorV1.CreateScriptResponse{
		Script: s.scriptRepo.ToProto(entity),
	}, nil
}

// GetScript retrieves a script by ID
func (s *ScriptService) GetScript(ctx context.Context, req *executorV1.GetScriptRequest) (*executorV1.GetScriptResponse, error) {
	entity, err := s.scriptRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorScriptNotFound("script not found")
	}

	return &executorV1.GetScriptResponse{
		Script: s.scriptRepo.ToProto(entity),
	}, nil
}

// ListScripts lists scripts with pagination and filters
func (s *ScriptService) ListScripts(ctx context.Context, req *executorV1.ListScriptsRequest) (*executorV1.ListScriptsResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	var page, pageSize uint32
	if req.Page != nil {
		page = *req.Page
	}
	if req.PageSize != nil {
		pageSize = *req.PageSize
	}

	var scriptType *string
	if req.ScriptType != nil && *req.ScriptType != executorV1.ScriptType_SCRIPT_TYPE_UNSPECIFIED {
		st := scriptTypeToString(*req.ScriptType)
		scriptType = &st
	}

	entities, total, err := s.scriptRepo.ListByTenant(ctx, tenantID, scriptType, req.Name, req.Enabled, page, pageSize)
	if err != nil {
		return nil, err
	}

	scripts := make([]*executorV1.Script, 0, len(entities))
	for _, e := range entities {
		scripts = append(scripts, s.scriptRepo.ToProto(e))
	}

	return &executorV1.ListScriptsResponse{
		Scripts: scripts,
		Total:   uint32(total),
	}, nil
}

// UpdateScript updates a script, requiring password when content changes
func (s *ScriptService) UpdateScript(ctx context.Context, req *executorV1.UpdateScriptRequest) (*executorV1.UpdateScriptResponse, error) {
	createdBy := getUserIDAsUint32(ctx)

	// Fetch existing script
	entity, err := s.scriptRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorScriptNotFound("script not found")
	}

	var newContentHash *string
	var newVersion *int

	// If content is changing, require password verification
	if req.Content != nil && *req.Content != entity.Content {
		if req.Password == nil || *req.Password == "" {
			return nil, executorV1.ErrorPasswordRequired("password is required when updating script content")
		}

		// Verify password via Portal
		username := getUsernameFromContext(ctx)
		if username == "" {
			return nil, executorV1.ErrorUnauthorized("cannot determine username for password verification")
		}

		verified, verifyErr := s.portalClient.VerifyCredential(ctx, username, *req.Password)
		if verifyErr != nil {
			s.log.Errorf("Password verification failed: %v", verifyErr)
			return nil, executorV1.ErrorPortalUnavailable("password verification service unavailable")
		}
		if !verified {
			return nil, executorV1.ErrorPasswordVerificationFailed("password verification failed")
		}

		hash := ComputeContentHash(*req.Content)
		newContentHash = &hash
		v := entity.Version + 1
		newVersion = &v
	}

	updated, err := s.scriptRepo.Update(ctx, req.Id, req.Name, req.Description, req.Content, newContentHash, req.Enabled, newVersion, createdBy)
	if err != nil {
		return nil, err
	}

	return &executorV1.UpdateScriptResponse{
		Script: s.scriptRepo.ToProto(updated),
	}, nil
}

// DeleteScript deletes a script and its assignments
func (s *ScriptService) DeleteScript(ctx context.Context, req *executorV1.DeleteScriptRequest) (*emptypb.Empty, error) {
	entity, err := s.scriptRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorScriptNotFound("script not found")
	}

	// Delete associated assignments first
	if delErr := s.assignRepo.DeleteByScriptID(ctx, req.Id); delErr != nil {
		s.log.Warnf("Failed to delete assignments for script %s: %v", req.Id, delErr)
	}

	if err := s.scriptRepo.Delete(ctx, req.Id); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// scriptTypeToString converts proto enum to ent string
func scriptTypeToString(t executorV1.ScriptType) string {
	switch t {
	case executorV1.ScriptType_SCRIPT_TYPE_BASH:
		return "BASH"
	case executorV1.ScriptType_SCRIPT_TYPE_JAVASCRIPT:
		return "JAVASCRIPT"
	case executorV1.ScriptType_SCRIPT_TYPE_LUA:
		return "LUA"
	default:
		return ""
	}
}
