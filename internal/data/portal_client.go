package data

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-common/registration"

	authV1 "github.com/go-tangra/go-tangra-portal/api/gen/go/authentication/service/v1"
)

// PortalClient wraps gRPC client for the Portal (admin-service) credential verification.
// It reuses the registration client's admin connection rather than creating its own,
// since admin-service is the same endpoint as the registration service.
type PortalClient struct {
	regClient *registration.Client
	log       *log.Helper

	once              sync.Once
	CredentialService authV1.UserCredentialServiceClient
	initErr           error
}

// NewPortalClient creates a new Portal gRPC client for credential verification.
// It accepts the registration client and reuses its admin connection.
func NewPortalClient(ctx *bootstrap.Context, regClient *registration.Client) (*PortalClient, func(), error) {
	l := ctx.NewLoggerHelper("portal/client/executor-service")

	client := &PortalClient{
		regClient: regClient,
		log:       l,
	}

	cleanup := func() {
		// Connection is owned by the registration client; nothing to close here.
	}

	l.Info("Portal client created (will initialize on first use)")
	return client, cleanup, nil
}

// resolve lazily creates the service client from the registration client's admin connection.
func (c *PortalClient) resolve() error {
	c.once.Do(func() {
		if c.regClient == nil {
			c.initErr = fmt.Errorf("registration client not available")
			c.log.Error("Registration client is nil, cannot connect to Portal")
			return
		}

		conn := c.regClient.AdminConn()
		if conn == nil {
			c.initErr = fmt.Errorf("admin connection not available from registration client")
			c.log.Error("Admin connection is nil")
			return
		}

		c.CredentialService = authV1.NewUserCredentialServiceClient(conn)
		c.log.Info("Portal client initialized via registration client's admin connection")
	})
	return c.initErr
}

// VerifyCredential verifies a user's password via the Portal's UserCredentialService
func (c *PortalClient) VerifyCredential(ctx context.Context, username, password string) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("portal client not available")
	}

	if err := c.resolve(); err != nil {
		return false, fmt.Errorf("portal client not ready: %w", err)
	}

	resp, err := c.CredentialService.VerifyCredential(ctx, &authV1.VerifyCredentialRequest{
		IdentityType: authV1.UserCredential_USERNAME,
		Identifier:   username,
		Credential:   password,
	})
	if err != nil {
		return false, fmt.Errorf("failed to verify credential via portal: %w", err)
	}
	return resp.GetSuccess(), nil
}
