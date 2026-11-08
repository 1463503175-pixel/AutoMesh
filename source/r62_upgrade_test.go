package main

import "testing"

func TestR62RecognizesReleasedR6Worker(t *testing.T) {
	const r6WorkerSHA256 = "d039bd4a5d70ea9ced93ff03a0652ba67f1b25f006295c984dfb547e30e3d35f"
	if !knownLegacyWorkerHashes[r6WorkerSHA256] {
		t.Fatal("R6 to R6.2 upgrade would reject a pristine worker")
	}
}
