package service

import (
	"testing"

	"github.com/go-tangra/go-tangra-executor/internal/data"
	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

func TestComputeActionHash_Deterministic(t *testing.T) {
	files := []data.ActionFileInput{
		{Path: "index.js", Content: "console.log(1)"},
		{Path: "lib/util.js", Content: "export const x = 1"},
	}
	h1 := computeActionHash("name: a", files)
	h2 := computeActionHash("name: a", files)
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64-char sha256 hex, got %d", len(h1))
	}
}

func TestComputeActionHash_OrderIndependent(t *testing.T) {
	a := []data.ActionFileInput{
		{Path: "index.js", Content: "a"},
		{Path: "b.js", Content: "b"},
	}
	b := []data.ActionFileInput{
		{Path: "b.js", Content: "b"},
		{Path: "index.js", Content: "a"},
	}
	if computeActionHash("m", a) != computeActionHash("m", b) {
		t.Fatal("hash should be independent of file order")
	}
}

func TestComputeActionHash_ContentSensitive(t *testing.T) {
	files := []data.ActionFileInput{{Path: "index.js", Content: "a"}}
	changed := []data.ActionFileInput{{Path: "index.js", Content: "b"}}
	if computeActionHash("m", files) == computeActionHash("m", changed) {
		t.Fatal("hash should change when file content changes")
	}
	if computeActionHash("m1", files) == computeActionHash("m2", files) {
		t.Fatal("hash should change when manifest changes")
	}
}

func TestProtoFilesToInput_SkipsEmptyPath(t *testing.T) {
	in := []*executorV1.ActionFile{
		{Path: "index.js", Content: "x"},
		{Path: "", Content: "ignored"},
	}
	out := protoFilesToInput(in)
	if len(out) != 1 || out[0].Path != "index.js" {
		t.Fatalf("expected 1 file (index.js), got %+v", out)
	}
}
