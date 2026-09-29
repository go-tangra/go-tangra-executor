package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
	"github.com/go-tangra/go-tangra-executor/internal/data"
)

type memSettings struct {
	rows map[string]data.StoredSetting
	fail error
}

func (m *memSettings) Get(_ context.Context, key string) (data.StoredSetting, bool, error) {
	if m.fail != nil {
		return data.StoredSetting{}, false, m.fail
	}
	r, ok := m.rows[key]
	return r, ok, nil
}

func (m *memSettings) Put(_ context.Context, key, value string, by uint32) error {
	if m.fail != nil {
		return m.fail
	}
	b := by
	m.rows[key] = data.StoredSetting{Value: value, UpdateTime: time.Now(), UpdatedBy: &b}
	return nil
}

func testCA(t *testing.T) string {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestInventoryAgentSettings(t *testing.T) {
	ctx := context.Background()
	store := &memSettings{rows: map[string]data.StoredSetting{}}
	s := &SettingsService{repo: store}

	got, err := s.GetInventoryAgentSettings(ctx, nil)
	if err != nil || got.Enabled || got.KeySecretConfigured || got.UpdateTime != nil {
		t.Fatalf("defaults %+v %v", got, err)
	}
	ca := testCA(t)
	req := &executorV1.UpdateInventoryAgentSettingsRequest{Enabled: true, IngestEndpoint: " portal.example.org:9977 ", KeyId: "ak_0123456789abcdef01234567",
		KeySecret: "aks_secret", CaPem: ca, ServerName: "portal.example.org", AgentVersion: "4.5.1"}
	got, err = s.UpdateInventoryAgentSettings(ctx, req)
	if err != nil || !got.Enabled || !got.KeySecretConfigured || got.IngestEndpoint != "portal.example.org:9977" || got.UpdateTime == nil {
		t.Fatalf("update %+v %v", got, err)
	}
	// The admin view never carries the secret.
	if strings.Contains(got.String(), "aks_secret") {
		t.Fatal("secret in the admin view")
	}
	// An empty secret keeps the stored one.
	req.KeySecret = ""
	if got, err = s.UpdateInventoryAgentSettings(ctx, req); err != nil || !got.KeySecretConfigured {
		t.Fatalf("keep secret %+v %v", got, err)
	}
	cfg, err := s.ClientConfig(ctx, "host-01", false)
	if err != nil || !cfg.Enabled || cfg.KeySecret != "aks_secret" || cfg.CaPem != strings.TrimSpace(ca) || cfg.AgentVersion != "4.5.1" {
		t.Fatalf("client config %+v %v", cfg, err)
	}
	// Only direct client certificates get it.
	for _, c := range []struct {
		cn string
		gw bool
	}{{"", false}, {"lcm-admin", false}, {"lcm-backup", false}, {"lcm-anything", false}, {"host-01", true}} {
		if _, err := s.ClientConfig(ctx, c.cn, c.gw); err == nil {
			t.Errorf("%q gateway=%v got the settings", c.cn, c.gw)
		}
	}
	// Enabled needs a secret; clearing it while enabled is refused.
	req.ClearKeySecret = true
	if _, err := s.UpdateInventoryAgentSettings(ctx, req); err == nil {
		t.Fatal("enabled without a secret accepted")
	}
	req.Enabled = false
	if got, err = s.UpdateInventoryAgentSettings(ctx, req); err != nil || got.KeySecretConfigured {
		t.Fatalf("clear %+v %v", got, err)
	}
	if cfg, _ := s.ClientConfig(ctx, "host-01", false); cfg.Enabled || cfg.KeySecret != "" {
		t.Fatalf("disabled config leaks %+v", cfg)
	}
}

func TestInventoryAgentValidation(t *testing.T) {
	ctx := context.Background()
	s := &SettingsService{repo: &memSettings{rows: map[string]data.StoredSetting{}}}
	base := func() *executorV1.UpdateInventoryAgentSettingsRequest {
		return &executorV1.UpdateInventoryAgentSettingsRequest{Enabled: true, IngestEndpoint: "h:9977", KeyId: "ak_0123456789abcdef01234567", KeySecret: "aks_x"}
	}
	bad := map[string]func(r *executorV1.UpdateInventoryAgentSettingsRequest){
		"endpoint":     func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.IngestEndpoint = "no-port" },
		"key id":       func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.KeyId = "ak_nothex" },
		"secret space": func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.KeySecret = "a b" },
		"secret long":  func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.KeySecret = strings.Repeat("x", 257) },
		"server name":  func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.ServerName = "bad name" },
		"version":      func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.AgentVersion = "newest" },
		"ca garbage":   func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.CaPem = "not pem" },
		"ca key": func(r *executorV1.UpdateInventoryAgentSettingsRequest) {
			r.CaPem = "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----"
		},
		"ca huge":    func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.CaPem = strings.Repeat("x", 65<<10) },
		"incomplete": func(r *executorV1.UpdateInventoryAgentSettingsRequest) { r.KeyId = "" },
	}
	for name, mut := range bad {
		r := base()
		mut(r)
		if _, err := s.UpdateInventoryAgentSettings(ctx, r); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	for _, v := range []string{"", "latest", "4.5.1", "v4.6.0"} {
		r := base()
		r.AgentVersion = v
		if _, err := s.UpdateInventoryAgentSettings(ctx, r); err != nil {
			t.Errorf("version %q: %v", v, err)
		}
	}
	// Disabled settings may be partial.
	if _, err := s.UpdateInventoryAgentSettings(ctx, &executorV1.UpdateInventoryAgentSettingsRequest{IngestEndpoint: "h:1"}); err != nil {
		t.Errorf("partial disabled: %v", err)
	}
}

func TestInventoryAgentStoreErrors(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("db down")
	s := &SettingsService{repo: &memSettings{rows: map[string]data.StoredSetting{}, fail: boom}}
	if _, err := s.GetInventoryAgentSettings(ctx, nil); err == nil {
		t.Error("get error swallowed")
	}
	if _, err := s.UpdateInventoryAgentSettings(ctx, &executorV1.UpdateInventoryAgentSettingsRequest{}); err == nil {
		t.Error("update error swallowed")
	}
	if _, err := s.ClientConfig(ctx, "h", false); err == nil {
		t.Error("client config error swallowed")
	}
	corrupt := &SettingsService{repo: &memSettings{rows: map[string]data.StoredSetting{inventoryAgentKey: {Value: "{"}}}}
	if _, err := corrupt.GetInventoryAgentSettings(ctx, nil); err == nil {
		t.Error("corrupt document accepted")
	}
}
