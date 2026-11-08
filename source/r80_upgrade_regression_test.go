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

func TestR802AcceptsExactR801AssetsForInPlaceUpgrade(t *testing.T) {
	const r801Worker = "5a9f10af7ad53b9ff168f52151375bfd87a456ec3c09b2627eb37dde745153c7"
	const r801Warning = "eaebbad9c9dac0b1a9e18d755ef19d0b236cfe8b8db444e4b46e2b62ffd6139a"
	if !knownLegacyWorkerHashes[r801Worker] {
		t.Fatal("exact R8.0.1 worker would be blocked during R8.0.2 upgrade")
	}
	if !knownLegacyAssetHashes["warning_refinement.py"][r801Warning] {
		t.Fatal("exact R8.0.1 warning module would be blocked during R8.0.2 upgrade")
	}
	if err := checkGeneratedAssetReplacement("meshing_worker.py", []byte("unrecognized user edit"), []byte("R8.0.2 worker")); err == nil {
		t.Fatal("unknown user-edited worker must remain protected")
	}
}

func TestR804AllowsExactR803AssetsForInPlaceUpgrade(t *testing.T) {
	if !knownLegacyWorkerHashes["c6bbc68afb28ac07add565021498544c29a4bcc4928d6bc8527f9bc866a991d6"] {
		t.Fatal("exact R8.0.3 worker would be blocked during R8.0.4 upgrade")
	}
	if !knownLegacyAssetHashes["warning_refinement.py"]["899fbef06a7285b03168ef935fde9394ce9c2d605180baa8920d59b9641711fd"] {
		t.Fatal("exact R8.0.3 warning module would be blocked during R8.0.4 upgrade")
	}
}
