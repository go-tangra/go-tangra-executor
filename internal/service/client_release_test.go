package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryNameFor(t *testing.T) {
	if got := BinaryNameFor("linux", "amd64"); got != "tangra-client-linux-amd64" {
		t.Fatalf("BinaryNameFor = %q", got)
	}
	if got := BinaryNameFor("darwin", "arm64"); got != "tangra-client-darwin-arm64" {
		t.Fatalf("BinaryNameFor = %q", got)
	}
}

func TestParseChecksumsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.sha256")
	content := "abc123  tangra-client-linux-amd64\n" +
		"def456 *tangra-client-darwin-arm64\n" +
		"badline\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	sums, err := parseChecksumsFile(path)
	if err != nil {
		t.Fatalf("parseChecksumsFile: %v", err)
	}
	if sums["tangra-client-linux-amd64"] != "abc123" {
		t.Errorf("linux amd64 = %q", sums["tangra-client-linux-amd64"])
	}
	// "*" binary-mode marker must be stripped from the filename.
	if sums["tangra-client-darwin-arm64"] != "def456" {
		t.Errorf("darwin arm64 = %q", sums["tangra-client-darwin-arm64"])
	}
	if len(sums) != 2 {
		t.Errorf("expected 2 entries, got %d", len(sums))
	}
}

func TestReleaseSnapshotAssetNil(t *testing.T) {
	var snap *releaseSnapshot
	if _, ok := snap.asset("anything"); ok {
		t.Error("nil snapshot should report no asset")
	}
}
