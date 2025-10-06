// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsEndpointValidation(t *testing.T) {
	tr := TaskRequest{AccessToken: verificationToken}
	if !tr.IsEndpointValidation() {
		t.Fatalf("expected IsEndpointValidation to be true")
	}
	tr.AccessToken = "not-token"
	if tr.IsEndpointValidation() {
		t.Fatalf("expected IsEndpointValidation to be false")
	}
}

func TestCreateRunTaskDirectoryStructure(t *testing.T) {
	t.Cleanup(func() { _ = os.RemoveAll("./tmp-workspace") })
	tr := TaskRequest{
		WorkspaceName: "tmp-workspace",
		RunID:         "run-123",
		Stage:         PrePlan,
	}
	path, err := tr.CreateRunTaskDirectoryStructure()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedSuffix := filepath.FromSlash("tmp-workspace/run-123/1_pre_plan")
	if filepath.Clean(path) != filepath.Clean("./"+expectedSuffix) {
		t.Fatalf("unexpected path: %s", path)
	}
	if tr.TaskDirectory == "" || tr.TaskDirectory != path {
		t.Fatalf("TaskDirectory not set correctly")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected directory to exist: %v", err)
	}
}
