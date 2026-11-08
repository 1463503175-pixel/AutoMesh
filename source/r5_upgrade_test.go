package main

import (
	"testing"
)

func TestR5UpgradeAllowsUnchangedR4Worker(t *testing.T) {
	// Known release worker must upgrade; local modifications must never be silently overwritten.
	const r4hash = "b249ffa220dc4ecf43048e94b5d6ab417b2c5e8f0980a67536ffd8a8edfcbecc"
	if !knownLegacyWorkerHashes[r4hash] {
		t.Fatal("R4 worker not on R5 upgrade allowlist")
	}
	if err := checkGeneratedAssetReplacement("meshing_worker.py", []byte("nonrelease-customization"), []byte("new")); err == nil {
		t.Fatal("customized worker must not be overwritten")
	}

}
