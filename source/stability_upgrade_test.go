package main

import "testing"

func TestR811AcceptsExactPreviousMainline(t *testing.T) {
	if !knownLegacyWorkerHashes["278cee13ed26a5d2892c119774ef9b4b97e8bdfe5249d3efa771b2326c0a75ec"] {
		t.Fatal("previous mainline worker would be blocked")
	}
	if !knownLegacyAssetHashes["warning_refinement.py"]["887372f1b6b6df0cce63f7a164e57562a0ead498b0ed755125dc6c834a52d115"] {
		t.Fatal("previous warning module would be blocked")
	}
}

func TestR812AcceptsPreviousBootstrapAssets(t *testing.T) {
	if !knownLegacyAssetHashes["start_wb_server_uia.ps1"]["e907731a6a7efac8aacf428125076e0ae9460c6bfa999df309d0c6d61fe83463"] {
		t.Fatal("previous bootstrap helper would be blocked")
	}
}
