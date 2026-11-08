package main

import "testing"

func TestR80AcceptsExactR79ReleaseScriptsOnly(t *testing.T) {
	const r79Worker = "7d4ec6d73f06a3c701535d71765f903b4d29be7a59d5b45bbf0bb66a49ca25e4"
	const r79Cluster = "c3c6e20c5a80eca010011630e07c9c2581dd2d0d254d7cd787871a7bfa00c39c"
	if !knownLegacyWorkerHashes[r79Worker] {
		t.Fatal("real R7.9 worker would be blocked by R8.0 initialization")
	}
	if !knownLegacyAssetHashes["warning_refinement.py"][r79Cluster] {
		t.Fatal("real R7.9 warning module would be blocked by R8.0 initialization")
	}
	if err := checkGeneratedAssetReplacement("meshing_worker.py", []byte("user-modified script"), []byte("R8 script")); err == nil {
		t.Fatal("unknown user scripts must not be silently overwritten")
	}
}
