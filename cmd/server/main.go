package main

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-common/registration"
	"github.com/go-tangra/go-tangra-common/service"
	"github.com/go-tangra/go-tangra-executor/cmd/server/assets"
	executorService "github.com/go-tangra/go-tangra-executor/internal/service"
)

var (
	moduleID    = "executor"
	moduleName  = "Executor"
	version     = "1.0.0"
	description = "Remote script execution management with hash verification and audit logging"
)

var globalRegHelper *registration.RegistrationHelper
var globalReleaseService *executorService.ClientReleaseService

func newApp(
	ctx *bootstrap.Context,
	gs *grpc.Server,
	hs *kratosHttp.Server,
	regClient *registration.Client,
	releaseService *executorService.ClientReleaseService,
) *kratos.App {
	// Start the client release poller and keep a reference for shutdown.
	globalReleaseService = releaseService
	if releaseService != nil {
		if err := releaseService.Start(); err != nil {
			ctx.NewLoggerHelper("executor/main").Warnf("Failed to start client release poller: %v", err)
		}
	}

	if regClient != nil {
		// Populate the full registration config on the pre-created client
		regClient.SetConfig(&registration.Config{
			ModuleID:         moduleID,
			ModuleName:       moduleName,
			Version:          version,
			Description:      description,
			GRPCEndpoint:     registration.GetGRPCAdvertiseAddr(ctx, "0.0.0.0:9800"),
			FrontendEntryUrl: registration.GetEnvOrDefault("FRONTEND_ENTRY_URL", ""),
			HttpEndpoint:     registration.GetEnvOrDefault("HTTP_ADVERTISE_ADDR", ""),
			OpenapiSpec:      assets.OpenApiData,
			ProtoDescriptor:  assets.DescriptorData,
			MenusYaml:        assets.MenusData,
		})
		globalRegHelper = registration.StartRegistrationWithClient(ctx.GetLogger(), regClient)
	}

	return bootstrap.NewApp(ctx, gs, hs)
}

func runApp() error {
	ctx := bootstrap.NewContext(
		context.Background(),
		&conf.AppInfo{
			Project: service.Project,
			AppId:   "executor.service",
			Version: version,
		},
	)

	defer func() {
		if globalRegHelper != nil {
			globalRegHelper.Stop()
		}
		if globalReleaseService != nil {
			_ = globalReleaseService.Stop()
		}
	}()

	return bootstrap.RunApp(ctx, initApp)
}

func main() {
	if err := runApp(); err != nil {
		panic(err)
	}
}
