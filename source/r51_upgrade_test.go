package main

import "testing"

func TestR51CanUpgradeR5Worker(t *testing.T) {
	oldHash := "0645a99ebac23328e9d5588bac3b82b8f1e7bf4a3b715e6e277fe2d1887d5c5e"
	if !knownLegacyWorkerHashes[oldHash] {
		t.Fatal("R5 embedded worker digest must be accepted for upgrade")
	}
	if got := friendlyACTError("RuntimeError: 审核生成前无法确认 Meshing 全局网格尺寸（可能显示 0.0 / 自动）"); got == "" {
		t.Fatal("required diagnostic missing")
	}
}
