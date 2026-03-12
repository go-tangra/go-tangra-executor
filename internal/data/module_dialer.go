package data

import (
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-common/registration"
)

// NewRegistrationClient creates a registration client connected to admin-service.
// This is created early (during Wire DI) so its admin connection can be shared
// with PortalClient for credential verification.
func NewRegistrationClient(ctx *bootstrap.Context) (*registration.Client, error) {
	adminEndpoint := os.Getenv("ADMIN_GRPC_ENDPOINT")
	if adminEndpoint == "" {
		return nil, nil
	}

	cfg := &registration.Config{
		AdminEndpoint: adminEndpoint,
		MaxRetries:    60,
	}

	return registration.NewClient(ctx.GetLogger(), cfg)
}

// RegistrationClientCleanup returns a cleanup function for the registration client.
func RegistrationClientCleanup(client *registration.Client) func() {
	return func() {
		if client != nil {
			_ = client.Close()
		}
	}
}

// ProvideRegistrationConfig builds the full registration config.
// This is used by main.go to start the registration lifecycle.
func ProvideRegistrationConfig(logger log.Logger, regClient *registration.Client) *RegistrationBundle {
	return &RegistrationBundle{
		Client: regClient,
		Logger: logger,
	}
}

// RegistrationBundle holds the registration client and logger for lifecycle management.
type RegistrationBundle struct {
	Client *registration.Client
	Logger log.Logger
}
