package main

import "testing"

func TestR741AcceptsVerifiedR74WorkerAndRejectsManualEdits(t *testing.T) {
	const verified = "1e5af1689430ac9c4c10ff63bee7065ca808b9bbe447251dfb37f6136e6b7b04"
	if !knownLegacyWorkerHashes[verified] {
		t.Fatal("unchanged R7.4 worker would block R7.4.1 startup")
	}
	if err := checkGeneratedAssetReplacement("meshing_worker.py", []byte("modified personal worker"), []byte("new released worker")); err == nil {
		t.Fatal("manually modified Worker must remain protected")
	}
}
