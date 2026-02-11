package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-common/middleware/mtls"
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
}

// NewClientService creates a new ClientService
func NewClientService(
	ctx *bootstrap.Context,
	scriptRepo *data.ScriptRepo,
	assignRepo *data.AssignmentRepo,
	execRepo *data.ExecutionLogRepo,
	cmdReg *CommandRegistry,
) *ClientService {
	return &ClientService{
		log:        ctx.NewLoggerHelper("executor/service/client"),
		scriptRepo: scriptRepo,
		assignRepo: assignRepo,
		execRepo:   execRepo,
		cmdReg:     cmdReg,
	}
}

// getClientCN extracts the client CN: first from gRPC metadata (admin-gateway proxied),
// then from the mTLS peer certificate context (direct client calls).
func getClientCN(ctx context.Context) string {
	if cn := getMetadataValue(ctx, "x-md-global-client-cn"); cn != "" {
		return cn
	}
	return mtls.GetClientID(ctx)
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
	// Fall back to req.ClientId if CN is unavailable.
	clientID := getClientCN(stream.Context())
	if clientID == "" {
		clientID = req.ClientId
	}
	s.log.Infof("Client %s (machine-id: %s) connected to command stream", clientID, req.ClientId)

	ch := s.cmdReg.Register(clientID)
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
