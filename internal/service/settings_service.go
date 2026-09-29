package service

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
	"github.com/go-tangra/go-tangra-executor/internal/data"
)

// inventoryAgentKey is the settings key of the v4 inventory agent settings.
const inventoryAgentKey = "inventory_agent"

// inventoryAgentDoc is the stored JSON document (the secret included).
type inventoryAgentDoc struct {
	Enabled        bool   `json:"enabled"`
	IngestEndpoint string `json:"ingest_endpoint"`
	KeyID          string `json:"key_id"`
	KeySecret      string `json:"key_secret"`
	CAPEM          string `json:"ca_pem"`
	ServerName     string `json:"server_name"`
	AgentVersion   string `json:"agent_version"`
}

var (
	autoKeyIDRE      = regexp.MustCompile(`^ak_[0-9a-f]{24}$`)
	agentVersionRE   = regexp.MustCompile(`^v?(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
	serverNameRE     = regexp.MustCompile(`^[A-Za-z0-9.-]{1,253}$`)
	maxCAPEMBytes    = 64 << 10
	maxKeySecretSize = 256
)

// SettingsService manages platform-wide client settings (admin API) and
// serves the inventory agent settings to clients.
type SettingsService struct {
	executorV1.UnimplementedExecutorSettingsServiceServer
	repo settingsStore
}

type settingsStore interface {
	Get(ctx context.Context, key string) (data.StoredSetting, bool, error)
	Put(ctx context.Context, key, value string, updatedBy uint32) error
}

// NewSettingsService creates a SettingsService.
func NewSettingsService(_ *bootstrap.Context, repo *data.SettingRepo) *SettingsService {
	return &SettingsService{repo: repo}
}

func (s *SettingsService) load(ctx context.Context) (inventoryAgentDoc, data.StoredSetting, error) {
	row, found, err := s.repo.Get(ctx, inventoryAgentKey)
	if err != nil || !found {
		return inventoryAgentDoc{}, row, err
	}
	var d inventoryAgentDoc
	if err := json.Unmarshal([]byte(row.Value), &d); err != nil {
		return inventoryAgentDoc{}, row, errors.InternalServer("SETTINGS_CORRUPT", "stored inventory agent settings are unreadable")
	}
	return d, row, nil
}

func view(d inventoryAgentDoc, row data.StoredSetting) *executorV1.InventoryAgentSettings {
	out := &executorV1.InventoryAgentSettings{
		Enabled: d.Enabled, IngestEndpoint: d.IngestEndpoint, KeyId: d.KeyID, KeySecretConfigured: d.KeySecret != "",
		CaPem: d.CAPEM, ServerName: d.ServerName, AgentVersion: d.AgentVersion, UpdatedBy: row.UpdatedBy,
	}
	if !row.UpdateTime.IsZero() {
		out.UpdateTime = timestamppb.New(row.UpdateTime)
	}
	return out
}

// GetInventoryAgentSettings returns the settings without the secret.
func (s *SettingsService) GetInventoryAgentSettings(ctx context.Context, _ *executorV1.GetInventoryAgentSettingsRequest) (*executorV1.InventoryAgentSettings, error) {
	d, row, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return view(d, row), nil
}

// UpdateInventoryAgentSettings validates and stores the settings.
func (s *SettingsService) UpdateInventoryAgentSettings(ctx context.Context, req *executorV1.UpdateInventoryAgentSettingsRequest) (*executorV1.InventoryAgentSettings, error) {
	cur, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	d := inventoryAgentDoc{
		Enabled:        req.GetEnabled(),
		IngestEndpoint: strings.TrimSpace(req.GetIngestEndpoint()),
		KeyID:          strings.TrimSpace(req.GetKeyId()),
		KeySecret:      cur.KeySecret,
		CAPEM:          strings.TrimSpace(req.GetCaPem()),
		ServerName:     strings.TrimSpace(req.GetServerName()),
		AgentVersion:   strings.TrimSpace(req.GetAgentVersion()),
	}
	if sec := strings.TrimSpace(req.GetKeySecret()); sec != "" {
		d.KeySecret = sec
	}
	if req.GetClearKeySecret() {
		d.KeySecret = ""
	}
	if err := validateInventoryAgent(d); err != nil {
		return nil, err
	}
	b, _ := json.Marshal(d)
	var by uint32
	if u := getUserIDAsUint32(ctx); u != nil {
		by = *u
	}
	if err := s.repo.Put(ctx, inventoryAgentKey, string(b), by); err != nil {
		return nil, err
	}
	return s.GetInventoryAgentSettings(ctx, nil)
}

func badRequest(msg string) error { return errors.BadRequest("INVALID_INVENTORY_AGENT_SETTINGS", msg) }

// validateInventoryAgent checks the values; enabled settings must be complete.
func validateInventoryAgent(d inventoryAgentDoc) error {
	if d.IngestEndpoint != "" {
		if host, port, err := net.SplitHostPort(d.IngestEndpoint); err != nil || host == "" || port == "" {
			return badRequest("ingest endpoint must be host:port, e.g. portal.example.org:9977")
		}
	}
	if d.KeyID != "" && !autoKeyIDRE.MatchString(d.KeyID) {
		return badRequest("key id must be ak_ followed by 24 hex characters")
	}
	if len(d.KeySecret) > maxKeySecretSize || strings.ContainsAny(d.KeySecret, " \t\r\n") {
		return badRequest("key secret must be a single token of at most 256 characters")
	}
	if d.ServerName != "" && !serverNameRE.MatchString(d.ServerName) {
		return badRequest("server name must be a DNS name")
	}
	if d.AgentVersion != "" && d.AgentVersion != "latest" && !agentVersionRE.MatchString(d.AgentVersion) {
		return badRequest("agent version must be empty, latest or a release such as 4.5.1")
	}
	if d.CAPEM != "" {
		if len(d.CAPEM) > maxCAPEMBytes {
			return badRequest("CA bundle is larger than 64 KiB")
		}
		if !validCABundle(d.CAPEM) {
			return badRequest("CA bundle must contain PEM certificates")
		}
	}
	if d.Enabled && (d.IngestEndpoint == "" || d.KeyID == "" || d.KeySecret == "") {
		return badRequest("to enable, set the ingest endpoint, the key id and the key secret")
	}
	return nil
}

func validCABundle(p string) bool {
	rest := []byte(p)
	n := 0
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		if b.Type != "CERTIFICATE" {
			return false
		}
		if _, err := x509.ParseCertificate(b.Bytes); err != nil {
			return false
		}
		n++
	}
	return n > 0 && strings.TrimSpace(string(rest)) == ""
}

// serviceIdentities are mTLS identities of platform services (never agent
// hosts); they never receive the inventory agent secret.
var serviceIdentities = map[string]bool{"lcm-admin": true, "lcm-backup": true, "lcm-scheduler": true}

// ClientConfig returns the settings for a client, secret included. Only a
// caller presenting its own client certificate directly (not through the
// admin gateway, not a service identity) gets them.
func (s *SettingsService) ClientConfig(ctx context.Context, peerCN string, viaGateway bool) (*executorV1.GetInventoryAgentConfigResponse, error) {
	if peerCN == "" || viaGateway || serviceIdentities[peerCN] || strings.HasPrefix(peerCN, "lcm-") {
		return nil, errors.Forbidden("CLIENT_CERT_REQUIRED", "only agent hosts receive the inventory agent settings")
	}
	d, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if !d.Enabled || d.IngestEndpoint == "" || d.KeyID == "" || d.KeySecret == "" {
		return &executorV1.GetInventoryAgentConfigResponse{Enabled: false}, nil
	}
	return &executorV1.GetInventoryAgentConfigResponse{Enabled: true, IngestEndpoint: d.IngestEndpoint, KeyId: d.KeyID, KeySecret: d.KeySecret,
		CaPem: d.CAPEM, ServerName: d.ServerName, AgentVersion: d.AgentVersion}, nil
}
