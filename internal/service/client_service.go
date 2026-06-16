package service

import (
	"context"
	"io"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/go-tangra/go-tangra-common/middleware/mtls"
	"github.com/go-tangra/go-tangra-common/viewer"
	"github.com/go-tangra/go-tangra-executor/internal/data"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

// ClientService implements the ExecutorClientService gRPC service (daemon-facing)
type ClientService struct {
	executorV1.UnimplementedExecutorClientServiceServer

	log        *log.Helper
	scriptRepo *data.ScriptRepo
	assignRepo *data.AssignmentRepo
	execRepo   *data.ExecutionLogRepo
	cmdReg     *CommandRegistry
	releaseSvc *ClientReleaseService
	actionRepo *data.ActionRepo
}

// NewClientService creates a new ClientService
func NewClientService(
	ctx *bootstrap.Context,
	scriptRepo *data.ScriptRepo,
	assignRepo *data.AssignmentRepo,
	execRepo *data.ExecutionLogRepo,
	cmdReg *CommandRegistry,
	releaseSvc *ClientReleaseService,
	actionRepo *data.ActionRepo,
) *ClientService {
	return &ClientService{
		log:        ctx.NewLoggerHelper("executor/service/client"),
		scriptRepo: scriptRepo,
		assignRepo: assignRepo,
		execRepo:   execRepo,
		cmdReg:     cmdReg,
		releaseSvc: releaseSvc,
		actionRepo: actionRepo,
	}
}

// getClientCN extracts the client CN: first from gRPC metadata (admin-gateway proxied),
// then from the mTLS peer certificate context (direct client calls via unary middleware).
func getClientCN(ctx context.Context) string {
	if cn := getMetadataValue(ctx, "x-md-global-client-cn"); cn != "" {
		return cn
	}
	return mtls.GetClientID(ctx)
}

// getPeerCN extracts the CN directly from the gRPC peer TLS certificate.
// This works for streaming RPCs where the Kratos unary mTLS middleware doesn't run.
func getPeerCN(ctx context.Context) string {
	// First try the middleware-set context (works for unary RPCs)
	if cn := getClientCN(ctx); cn != "" {
		return cn
	}
	// Fall back to extracting directly from peer TLS info (works for streaming RPCs)
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return ""
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 {
		return ""
	}
	return tlsInfo.State.PeerCertificates[0].Subject.CommonName
}

// FetchScript returns script content + hash, validating that the client is assigned
func (s *ClientService) FetchScript(ctx context.Context, req *executorV1.FetchScriptRequest) (*executorV1.FetchScriptResponse, error) {
	clientCN := getClientCN(ctx)

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

	// Validate assignment — check by mTLS CN
	if clientCN != "" {
		assigned, aErr := s.assignRepo.ExistsAnyTenant(ctx, req.ScriptId, clientCN)
		if aErr != nil {
			return nil, aErr
		}
		if !assigned {
			return nil, executorV1.ErrorClientNotAssigned("script is not assigned to this client")
		}
	}

	return &executorV1.FetchScriptResponse{
		ScriptId:   script.ID,
		ScriptName: script.Name,
		ScriptType: scriptTypeToProto(string(script.ScriptType)),
		Content:    script.Content,
		ContentHash: script.ContentHash,
		Version:    int32(script.Version),
	}, nil
}

// StreamCommands opens a server-side stream for the client to receive execution commands
func (s *ClientService) StreamCommands(req *executorV1.StreamCommandsRequest, stream executorV1.ExecutorClientService_StreamCommandsServer) error {
	// Use the mTLS CN as the registry key so it matches TriggerExecution lookups.
	// getPeerCN extracts CN directly from the TLS peer cert, which works for
	// streaming RPCs where the Kratos unary mTLS middleware doesn't run.
	clientID := getPeerCN(stream.Context())
	if clientID == "" {
		clientID = req.ClientId
	}
	s.log.Infof("Client %s (machine-id: %s) connected to command stream (version: %s)", clientID, req.ClientId, req.GetClientVersion())

	ch := s.cmdReg.Register(clientID, req.GetClientVersion(), req.GetActionsEnabled())
	defer func() {
		s.cmdReg.Unregister(clientID)
		s.log.Infof("Client %s disconnected from command stream", clientID)
	}()

	for {
		select {
		case cmd, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(cmd); err != nil {
				s.log.Errorf("Failed to send command to client %s: %v", clientID, err)
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// AckCommand acknowledges a command (accepted or rejected)
func (s *ClientService) AckCommand(ctx context.Context, req *executorV1.AckCommandRequest) (*executorV1.AckCommandResponse, error) {
	if req.Accepted {
		// Client accepted — it will execute and call ReportResult
		return &executorV1.AckCommandResponse{Acknowledged: true}, nil
	}

	// Client rejected — determine the rejection status
	reason := ""
	if req.RejectionReason != nil {
		reason = *req.RejectionReason
	}

	status := "REJECTED_NOT_APPROVED"
	if reason == "hash_mismatch" {
		status = "REJECTED_HASH_MISMATCH"
	}

	// The command_id can be used to map back to an execution log
	// For now, we use the command_id as the execution_id lookup
	// The TriggerExecution flow stores execution_id in the command
	// Client should send the execution_id via a separate field or use command_id
	// Since our AckCommand only has command_id, we log the rejection
	s.log.Infof("Command %s rejected by client: %s (reason: %s)", req.CommandId, status, reason)

	return &executorV1.AckCommandResponse{Acknowledged: true}, nil
}

// SubmitExecution creates a complete execution log in one shot (client-pull scenario)
func (s *ClientService) SubmitExecution(ctx context.Context, req *executorV1.SubmitExecutionRequest) (*executorV1.SubmitExecutionResponse, error) {
	clientCN := getClientCN(ctx)

	// Look up the script
	script, err := s.scriptRepo.GetByID(ctx, req.ScriptId)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, executorV1.ErrorScriptNotFound("script not found")
	}

	// Validate assignment
	if clientCN != "" {
		assigned, aErr := s.assignRepo.ExistsAnyTenant(ctx, req.ScriptId, clientCN)
		if aErr != nil {
			return nil, aErr
		}
		if !assigned {
			return nil, executorV1.ErrorClientNotAssigned("script is not assigned to this client")
		}
	}

	// Determine status from exit code
	status := "COMPLETED"
	if req.ExitCode != 0 {
		status = "FAILED"
	}

	tenantID := uint32(0)
	if script.TenantID != nil {
		tenantID = *script.TenantID
	}

	// Create execution log
	execLog, err := s.execRepo.Create(
		ctx, tenantID, script.ID, script.Name,
		clientCN, script.ContentHash, "CLIENT_PULL", status, nil,
	)
	if err != nil {
		return nil, err
	}

	// Store result
	if err := s.execRepo.UpdateResult(ctx, execLog.ID, int(req.ExitCode), req.Output, req.ErrorOutput, req.DurationMs); err != nil {
		return nil, err
	}

	return &executorV1.SubmitExecutionResponse{
		ExecutionId: execLog.ID,
		Recorded:    true,
	}, nil
}

// ReportResult stores execution results from the client
func (s *ClientService) ReportResult(ctx context.Context, req *executorV1.ReportResultRequest) (*executorV1.ReportResultResponse, error) {
	entity, err := s.execRepo.GetByID(ctx, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, executorV1.ErrorExecutionNotFound("execution not found")
	}

	if err := s.execRepo.UpdateResult(ctx, req.ExecutionId, int(req.ExitCode), req.Output, req.ErrorOutput, req.DurationMs); err != nil {
		return nil, err
	}

	return &executorV1.ReportResultResponse{Recorded: true}, nil
}

// downloadChunkSize is the size of each ClientBinaryChunk sent over the stream.
// Kept well under the default gRPC 4MiB message limit.
const downloadChunkSize = 256 * 1024

// GetLatestClientRelease returns metadata about the latest client binary the
// executor has cached for the requested platform.
func (s *ClientService) GetLatestClientRelease(_ context.Context, req *executorV1.GetLatestClientReleaseRequest) (*executorV1.GetLatestClientReleaseResponse, error) {
	if s.releaseSvc == nil {
		return &executorV1.GetLatestClientReleaseResponse{Available: false}, nil
	}

	snapshot := s.releaseSvc.Latest()
	if snapshot == nil {
		return &executorV1.GetLatestClientReleaseResponse{Available: false}, nil
	}

	binaryName := BinaryNameFor(req.GetOs(), req.GetArch())
	asset, ok := snapshot.asset(binaryName)
	if !ok {
		// We have a release cached, but not a binary for this platform.
		return &executorV1.GetLatestClientReleaseResponse{
			Available:  false,
			Version:    snapshot.Version,
			ReleaseUrl: snapshot.ReleaseURL,
		}, nil
	}

	return &executorV1.GetLatestClientReleaseResponse{
		Available:  true,
		Version:    snapshot.Version,
		BinaryName: asset.Name,
		Sha256:     asset.SHA256,
		Size:       asset.Size,
		ReleaseUrl: snapshot.ReleaseURL,
	}, nil
}

// DownloadClientBinary streams a cached client binary to the caller in chunks.
func (s *ClientService) DownloadClientBinary(req *executorV1.DownloadClientBinaryRequest, stream executorV1.ExecutorClientService_DownloadClientBinaryServer) error {
	if s.releaseSvc == nil {
		return executorV1.ErrorScriptNotFound("client release cache is not available")
	}

	reader, asset, err := s.releaseSvc.OpenAsset(req.GetBinaryName())
	if err != nil {
		return executorV1.ErrorScriptNotFound("binary not cached: %v", err)
	}
	defer reader.Close()

	s.log.Infof("Streaming client binary %q (%d bytes) to client", asset.Name, asset.Size)

	buf := make([]byte, downloadChunkSize)
	for {
		n, rErr := reader.Read(buf)
		if n > 0 {
			if sErr := stream.Send(&executorV1.ClientBinaryChunk{Data: buf[:n]}); sErr != nil {
				return sErr
			}
		}
		if rErr == io.EOF {
			return nil
		}
		if rErr != nil {
			return rErr
		}
	}
}

// ResolveAction returns an action package (manifest + files) by name so the
// go-tangra-actions engine can execute it on the host. Resolves within the
// caller's tenant scope.
func (s *ClientService) ResolveAction(ctx context.Context, req *executorV1.ResolveActionRequest) (*executorV1.ResolveActionResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	entity, err := s.actionRepo.GetByName(ctx, tenantID, req.GetName())
	if err != nil {
		return nil, err
	}
	if entity == nil || !entity.Enabled {
		return nil, executorV1.ErrorNotFound("action %q not found", req.GetName())
	}

	return &executorV1.ResolveActionResponse{
		Name:     entity.Name,
		Manifest: entity.Manifest,
		Files:    s.actionRepo.ActionFilesToProto(entity.Edges.Files),
		Version:  int32(entity.Version),
	}, nil
}

// StreamExecutionOutput receives live workflow output chunks from a client and
// appends them to the execution log, so logs are captured as they are produced.
func (s *ClientService) StreamExecutionOutput(stream executorV1.ExecutorClientService_StreamExecutionOutputServer) error {
	// Streaming RPCs bypass the unary middleware chain, so the system viewer
	// context that ent's tenant privacy policies require is not present. Inject
	// it here, the same way systemViewerMiddleware does for unary calls —
	// otherwise the execution-log queries below fail the privacy policy.
	ctx := viewer.NewSystemViewerContext(stream.Context())
	var (
		execID string
		chunks int64
	)
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&executorV1.StreamExecutionOutputResponse{Recorded: true, Chunks: chunks})
		}
		if err != nil {
			return err
		}
		if chunk.GetExecutionId() == "" {
			continue
		}
		// Mark the execution running on the first chunk.
		if execID == "" {
			execID = chunk.GetExecutionId()
			if sErr := s.execRepo.SetStartedAt(ctx, execID); sErr != nil {
				s.log.Warnf("failed to mark execution %s running: %v", execID, sErr)
			}
		}
		if len(chunk.GetData()) == 0 {
			continue
		}
		if aErr := s.execRepo.AppendOutput(ctx, chunk.GetExecutionId(), string(chunk.GetData())); aErr != nil {
			s.log.Errorf("append output for execution %s failed: %v", chunk.GetExecutionId(), aErr)
			return aErr
		}
		chunks++
	}
}
