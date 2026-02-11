package data

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	authV1 "github.com/go-tangra/go-tangra-portal/api/gen/go/authentication/service/v1"
)

// PortalClient wraps gRPC client for the Portal (admin-service) credential verification
type PortalClient struct {
	conn              *grpc.ClientConn
	log               *log.Helper
	CredentialService authV1.UserCredentialServiceClient
}

// NewPortalClient creates a new Portal gRPC client for credential verification
func NewPortalClient(ctx *bootstrap.Context) (*PortalClient, func(), error) {
	l := ctx.NewLoggerHelper("portal/client/executor-service")

	endpoint := getEnvOrDefault("PORTAL_GRPC_ENDPOINT", "localhost:7787")

	l.Infof("Connecting to Portal service at: %s", endpoint)

	var dialOpt grpc.DialOption
	creds, err := loadPortalClientTLSCredentials(l)
	if err != nil {
		l.Warnf("Failed to load TLS credentials for Portal, using insecure: %v", err)
		dialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
	} else {
		dialOpt = grpc.WithTransportCredentials(creds)
	}

	connectParams := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  1 * time.Second,
			Multiplier: 1.5,
			Jitter:     0.2,
			MaxDelay:   30 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}

	keepaliveParams := keepalive.ClientParameters{
		Time:                5 * time.Minute,
		Timeout:             20 * time.Second,
		PermitWithoutStream: false,
	}

	conn, err := grpc.NewClient(
		endpoint,
		dialOpt,
		grpc.WithConnectParams(connectParams),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithDefaultServiceConfig(`{
			"loadBalancingConfig": [{"round_robin":{}}],
			"methodConfig": [{
				"name": [{"service": ""}],
				"waitForReady": true,
				"retryPolicy": {
					"MaxAttempts": 3,
					"InitialBackoff": "0.5s",
					"MaxBackoff": "5s",
					"BackoffMultiplier": 2,
					"RetryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED"]
				}
			}]
		}`),
	)
	if err != nil {
		l.Errorf("Failed to connect to Portal service: %v", err)
		return nil, func() {}, err
	}

	client := &PortalClient{
		conn:              conn,
		log:               l,
		CredentialService: authV1.NewUserCredentialServiceClient(conn),
	}

	cleanup := func() {
		if err := conn.Close(); err != nil {
			l.Errorf("Failed to close Portal connection: %v", err)
		}
	}

	l.Info("Portal client initialized successfully")
	return client, cleanup, nil
}

// VerifyCredential verifies a user's password via the Portal's UserCredentialService
func (c *PortalClient) VerifyCredential(ctx context.Context, username, password string) (bool, error) {
	if c == nil || c.conn == nil {
		return false, fmt.Errorf("portal client not available")
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

func loadPortalClientTLSCredentials(l *log.Helper) (credentials.TransportCredentials, error) {
	caCertPath := os.Getenv("PORTAL_CA_CERT_PATH")
	if caCertPath == "" {
		caCertPath = "./data/ca/ca.crt"
	}
	clientCertPath := os.Getenv("PORTAL_CLIENT_CERT_PATH")
	if clientCertPath == "" {
		clientCertPath = "./data/portal/portal.crt"
	}
	clientKeyPath := os.Getenv("PORTAL_CLIENT_KEY_PATH")
	if clientKeyPath == "" {
		clientKeyPath = "./data/portal/portal.key"
	}

	serverName := os.Getenv("PORTAL_SERVER_NAME")
	if serverName == "" {
		serverName = "admin-service"
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert from %s: %w", caCertPath, err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert/key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}
