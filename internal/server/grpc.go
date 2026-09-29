package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metadata"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	commonV1 "github.com/go-tangra/go-tangra-common/gen/go/common/service/v1"

	"github.com/go-tangra/go-tangra-common/viewer"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
	"github.com/go-tangra/go-tangra-executor/internal/cert"
	"github.com/go-tangra/go-tangra-executor/internal/metrics"
	"github.com/go-tangra/go-tangra-executor/internal/service"

	"github.com/go-tangra/go-tangra-common/middleware/audit"
	"github.com/go-tangra/go-tangra-common/middleware/claims"
	"github.com/go-tangra/go-tangra-common/middleware/mtls"
)

// systemViewerMiddleware injects system viewer context for all requests
func systemViewerMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			ctx = viewer.NewSystemViewerContext(ctx)
			return handler(ctx, req)
		}
	}
}

// NewGRPCServer creates a gRPC server with mTLS and audit logging
func NewGRPCServer(
	ctx *bootstrap.Context,
	certManager *cert.CertManager,
	collector *metrics.Collector,
	scriptSvc *service.ScriptService,
	actionSvc *service.ActionService,
	workflowSvc *service.WorkflowService,
	assignSvc *service.AssignmentService,
	execSvc *service.ExecutionService,
	clientSvc *service.ClientService,
	statsSvc *service.StatisticsService,
	backupSvc *service.BackupService,
	sqlBackupSvc *service.SqlBackupService,
	settingsSvc *service.SettingsService,
) *grpc.Server {
	cfg := ctx.GetConfig()
	l := ctx.NewLoggerHelper("executor/grpc")

	var opts []grpc.ServerOption

	if cfg.Server != nil && cfg.Server.Grpc != nil {
		if cfg.Server.Grpc.Network != "" {
			opts = append(opts, grpc.Network(cfg.Server.Grpc.Network))
		}
		if cfg.Server.Grpc.Addr != "" {
			opts = append(opts, grpc.Address(cfg.Server.Grpc.Addr))
		}
		if cfg.Server.Grpc.Timeout != nil {
			opts = append(opts, grpc.Timeout(cfg.Server.Grpc.Timeout.AsDuration()))
		}
	}

	// Configure TLS if certificates are available
	if certManager != nil && certManager.IsTLSEnabled() {
		tlsConfig, err := certManager.GetServerTLSConfig()
		if err != nil {
			l.Warnf("Failed to get TLS config, running without TLS: %v", err)
		} else {
			opts = append(opts, grpc.TLSConfig(tlsConfig))
			l.Info("gRPC server configured with mTLS")
		}
	} else {
		l.Warn("TLS not enabled, running without mTLS")
	}

	// Add middleware
	var ms []middleware.Middleware
	ms = append(ms, recovery.Recovery())
	ms = append(ms, collector.Middleware())
	ms = append(ms, systemViewerMiddleware())
	ms = append(ms, tracing.Server())
	ms = append(ms, metadata.Server())
	ms = append(ms, logging.Server(ctx.GetLogger()))

	// Add mTLS middleware only when TLS is enabled
	if certManager != nil && certManager.IsTLSEnabled() {
		ms = append(ms, mtls.MTLSMiddleware(
			ctx.GetLogger(),
			mtls.WithPublicEndpoints(
				"/grpc.health.v1.Health/Check",
				"/grpc.health.v1.Health/Watch",
			),
			// mTLS caller allow-list (client cert CN "lcm-<module>"); gateway=lcm-admin, backup=lcm-backup.
			// scheduler dispatches task executions; its admin-cert fallback presents lcm-admin.
			mtls.WithAllowedIdentities("lcm-admin", "lcm-backup", "lcm-scheduler"),
		))
	}

	// Bind x-md-global-* user claims to the gateway's HMAC assertion so a direct
	// mTLS caller cannot forge platform:admin (CRIT-3.2). No-op for calls that
	// carry no user claims; strips unverified claims in enforce mode.
	ms = append(ms, claims.Server(ctx.GetLogger()))

	ms = append(ms, audit.Server(
		ctx.GetLogger(),
		audit.WithServiceName("executor-service"),
		audit.WithSkipOperations(
			"/grpc.health.v1.Health/Check",
			"/grpc.health.v1.Health/Watch",
			"/executor.service.v1.BackupService/ExportBackup",
			"/executor.service.v1.BackupService/ImportBackup",
		),
	))

	ms = append(ms, validate.Validator())

	opts = append(opts, grpc.Middleware(ms...))

	srv := grpc.NewServer(opts...)

	// Register services with redacted wrappers to prevent sensitive data from leaking in logs
	executorV1.RegisterRedactedExecutorScriptServiceServer(srv, scriptSvc, nil)
	executorV1.RegisterRedactedExecutorActionServiceServer(srv, actionSvc, nil)
	executorV1.RegisterRedactedExecutorWorkflowServiceServer(srv, workflowSvc, nil)
	executorV1.RegisterRedactedExecutorAssignmentServiceServer(srv, assignSvc, nil)
	executorV1.RegisterRedactedExecutorExecutionServiceServer(srv, execSvc, nil)
	executorV1.RegisterRedactedExecutorClientServiceServer(srv, clientSvc, nil)
	executorV1.RegisterRedactedExecutorStatisticsServiceServer(srv, statsSvc, nil)
	executorV1.RegisterRedactedBackupServiceServer(srv, backupSvc, nil)
	commonV1.RegisterBackupServiceServer(srv, sqlBackupSvc)
	executorV1.RegisterRedactedExecutorSettingsServiceServer(srv, settingsSvc, nil)

	return srv
}
