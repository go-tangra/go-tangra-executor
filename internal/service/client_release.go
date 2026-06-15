package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

const (
	defaultReleaseRepo  = "go-tangra/go-tangra-client"
	defaultPollInterval = 30 * time.Minute
	releaseBinaryPrefix = "tangra-client"
	checksumAssetName   = "tangra-client-checksums.sha256"
	releasePollTimeout  = 10 * time.Minute
)

// cachedAsset describes one downloaded release asset on disk.
type cachedAsset struct {
	Name   string
	Path   string
	Size   int64
	SHA256 string // hex-encoded; empty when the asset is not listed in the checksum file
}

// releaseSnapshot is an immutable view of the release currently cached on disk.
// It is swapped atomically so readers never see a half-populated cache.
type releaseSnapshot struct {
	Version    string
	ReleaseURL string
	Assets     map[string]*cachedAsset
}

func (r *releaseSnapshot) asset(name string) (*cachedAsset, bool) {
	if r == nil {
		return nil, false
	}
	a, ok := r.Assets[name]
	return a, ok
}

// ClientReleaseService polls GitHub for the latest go-tangra-client release,
// caches its assets on disk, and serves them to connected clients over the
// existing mTLS channel. Centralizing the GitHub poll here means an entire
// fleet of clients updates through the executor without each one hitting the
// GitHub API (which is rate-limited to 60 requests/hour for anonymous callers).
type ClientReleaseService struct {
	log      *log.Helper
	repo     string
	token    string
	interval time.Duration
	cacheDir string
	enabled  bool

	current atomic.Pointer[releaseSnapshot]

	stopCh      chan struct{}
	wg          sync.WaitGroup
	running     bool
	runningLock sync.Mutex
}

// NewClientReleaseService constructs the service from environment configuration.
//
//	CLIENT_UPDATE_REPO         GitHub "owner/repo" (default go-tangra/go-tangra-client)
//	CLIENT_UPDATE_POLL_INTERVAL  poll cadence as a Go duration (default 30m)
//	CLIENT_RELEASE_CACHE_DIR   on-disk cache (default <tmp>/tangra-client-releases)
//	GITHUB_TOKEN               optional token to raise the GitHub rate limit
//	CLIENT_UPDATE_DISABLED     set to "true"/"1" to disable the poller entirely
func NewClientReleaseService(ctx *bootstrap.Context) *ClientReleaseService {
	repo := getEnvOr("CLIENT_UPDATE_REPO", defaultReleaseRepo)

	interval := defaultPollInterval
	if raw := os.Getenv("CLIENT_UPDATE_POLL_INTERVAL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			interval = d
		}
	}

	cacheDir := os.Getenv("CLIENT_RELEASE_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "tangra-client-releases")
	}

	disabled := false
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CLIENT_UPDATE_DISABLED"))) {
	case "1", "true", "yes":
		disabled = true
	}

	return &ClientReleaseService{
		log:      ctx.NewLoggerHelper("executor/service/client-release"),
		repo:     repo,
		token:    os.Getenv("GITHUB_TOKEN"),
		interval: interval,
		cacheDir: cacheDir,
		enabled:  !disabled,
	}
}

func getEnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Start launches the background poller. Safe to call once.
func (s *ClientReleaseService) Start() error {
	s.runningLock.Lock()
	defer s.runningLock.Unlock()

	if s.running {
		return nil
	}
	if !s.enabled {
		s.log.Info("Client release poller is disabled (CLIENT_UPDATE_DISABLED)")
		return nil
	}

	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating client release cache dir: %w", err)
	}

	s.stopCh = make(chan struct{})
	s.running = true

	s.log.Infof("Starting client release poller: repo=%s interval=%s cache=%s",
		s.repo, s.interval, s.cacheDir)

	s.wg.Add(1)
	go s.run()

	return nil
}

// Stop halts the background poller.
func (s *ClientReleaseService) Stop() error {
	s.runningLock.Lock()
	defer s.runningLock.Unlock()

	if !s.running {
		return nil
	}
	close(s.stopCh)
	s.wg.Wait()
	s.running = false
	s.log.Info("Client release poller stopped")
	return nil
}

func (s *ClientReleaseService) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Poll immediately on startup so the cache is warm as soon as possible.
	s.pollOnce()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.pollOnce()
		}
	}
}

func (s *ClientReleaseService) pollOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), releasePollTimeout)
	defer cancel()

	if err := s.poll(ctx); err != nil {
		s.log.Warnf("Client release poll failed: %v", err)
	}
}

// Latest returns the currently cached release snapshot, or nil if none is cached yet.
func (s *ClientReleaseService) Latest() *releaseSnapshot {
	return s.current.Load()
}

// AssetFor returns the cached asset with the given name, if present.
func (s *ClientReleaseService) AssetFor(name string) (*cachedAsset, bool) {
	return s.current.Load().asset(name)
}

// OpenAsset opens a cached asset file for reading.
func (s *ClientReleaseService) OpenAsset(name string) (io.ReadCloser, *cachedAsset, error) {
	asset, ok := s.AssetFor(name)
	if !ok {
		return nil, nil, fmt.Errorf("asset %q not cached", name)
	}
	f, err := os.Open(asset.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening cached asset %q: %w", name, err)
	}
	return f, asset, nil
}

// poll fetches the latest release metadata and, if newer than what is cached,
// downloads and verifies every asset, then atomically swaps in the new snapshot.
func (s *ClientReleaseService) poll(ctx context.Context) error {
	release, err := s.fetchLatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("fetching latest release: %w", err)
	}

	version := strings.TrimPrefix(release.TagName, "v")
	if version == "" {
		return fmt.Errorf("release has empty tag")
	}

	if cur := s.current.Load(); cur != nil && cur.Version == version {
		return nil // already cached
	}

	s.log.Infof("New client release detected: %s (%d assets)", version, len(release.Assets))

	versionDir := filepath.Join(s.cacheDir, version)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return fmt.Errorf("creating version dir: %w", err)
	}

	// Download all assets first so we have the checksum file available.
	assets := make(map[string]*cachedAsset, len(release.Assets))
	for _, a := range release.Assets {
		if a.Name == "" || a.BrowserDownloadURL == "" {
			continue
		}
		dst := filepath.Join(versionDir, filepath.Base(a.Name))
		size, sum, dErr := s.downloadAsset(ctx, a.BrowserDownloadURL, dst)
		if dErr != nil {
			return fmt.Errorf("downloading asset %q: %w", a.Name, dErr)
		}
		assets[a.Name] = &cachedAsset{Name: a.Name, Path: dst, Size: size, SHA256: sum}
	}

	// Cross-check binary checksums against the published checksum file.
	if cs, ok := assets[checksumAssetName]; ok {
		published, pErr := parseChecksumsFile(cs.Path)
		if pErr != nil {
			s.log.Warnf("Failed to parse checksum file: %v", pErr)
		} else {
			for name, asset := range assets {
				want, listed := published[name]
				if !listed {
					continue
				}
				if !strings.EqualFold(want, asset.SHA256) {
					return fmt.Errorf("checksum mismatch for %q: published %s, downloaded %s", name, want, asset.SHA256)
				}
			}
		}
	}

	snapshot := &releaseSnapshot{
		Version:    version,
		ReleaseURL: release.HTMLURL,
		Assets:     assets,
	}
	s.current.Store(snapshot)
	s.log.Infof("Cached client release %s (%d assets) in %s", version, len(assets), versionDir)

	s.pruneOldVersions(version)
	return nil
}

// pruneOldVersions removes cached version directories other than the current one.
func (s *ClientReleaseService) pruneOldVersions(keep string) {
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == keep {
			continue
		}
		if rmErr := os.RemoveAll(filepath.Join(s.cacheDir, e.Name())); rmErr != nil {
			s.log.Warnf("Failed to prune old release %q: %v", e.Name(), rmErr)
		}
	}
}

// downloadAsset streams a URL to dst and returns its size and hex SHA-256.
func (s *ClientReleaseService) downloadAsset(ctx context.Context, url, dst string) (int64, string, error) {
	resp, err := s.httpGetWithRetry(ctx, url, nil)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("status %d", resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	HTMLURL string    `json:"html_url"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func (s *ClientReleaseService) fetchLatestRelease(ctx context.Context) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", s.repo)
	resp, err := s.httpGetWithRetry(ctx, url, map[string]string{"Accept": "application/vnd.github+json"})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}
	return &release, nil
}

func releaseTransientStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// httpGetWithRetry performs a GET with a few backoff retries on network errors
// and transient HTTP statuses. A GITHUB_TOKEN, when set, is sent as a bearer
// token to raise the rate limit.
func (s *ClientReleaseService) httpGetWithRetry(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	const maxAttempts = 4
	backoff := time.Second
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if s.token != "" {
			req.Header.Set("Authorization", "Bearer "+s.token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
		} else if releaseTransientStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			resp.Body.Close()
		} else {
			return resp, nil
		}

		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return nil, fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
}

// parseChecksumsFile reads a sha256sum-format file and returns name -> hex hash.
func parseChecksumsFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// sha256sum format: "<hash>  <filename>" ("*" marks binary mode)
		name := strings.TrimPrefix(fields[1], "*")
		out[name] = fields[0]
	}
	return out, nil
}

// BinaryNameFor returns the release asset name for a platform.
func BinaryNameFor(goos, goarch string) string {
	return fmt.Sprintf("%s-%s-%s", releaseBinaryPrefix, goos, goarch)
}
