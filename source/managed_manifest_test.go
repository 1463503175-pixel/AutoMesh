package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifiedManifestUsesProjectAndGeometryNotNameAlone(t *testing.T) {
	d := t.TempDir()
	s := &Snapshot{Project: "E:/FFF", Fingerprint: "abcdef"}
	if hasVerifiedManagedManifest(d, s) {
		t.Fatal("missing manifest was accepted")
	}
	m := map[string]any{"status": "ready", "project": s.Project, "fingerprint": s.Fingerprint, "prepared": []any{map[string]any{"ns": "AM_A", "sizing": "AutoMesh_NS_AM_A"}}}
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(d, "editable_manifest.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if !hasVerifiedManagedManifest(d, s) {
		t.Fatal("matching manifest not found")
	}
	s.Fingerprint = "different"
	if hasVerifiedManagedManifest(d, s) {
		t.Fatal("stale fingerprint accepted")
	}
}
func TestReportQualityAdviceIsNotFaked(t *testing.T) {
	q := qualityAdvice(map[string]any{})
	if q == "" {
		t.Fatal("no diagnostic note on absent mesh")
	}
	q = qualityAdvice(map[string]any{"quality": map[string]any{"skewness": map[string]any{"max": float64(.96)}}})
	if q == "" {
		t.Fatal("missing severe-skewness guidance")
	}
}

func TestOnlyNativeManagedControlsUnlockEmptyRepublish(t *testing.T) {
	d := t.TempDir()
	snap := &Snapshot{Project: "E:/FFF", Fingerprint: "abcd"}
	name := "AM_细孔组"
	native := expectedManagedSizingName(name)
	for _, tc := range []struct {
		sizing string
		want   bool
	}{{"AutoMesh_legacy", false}, {native, true}} {
		m := map[string]any{"status": "ready", "project": snap.Project, "fingerprint": snap.Fingerprint,
			"prepared": []any{map[string]any{"ns": name, "sizing": tc.sizing}}}
		raw, _ := json.Marshal(m)
		if err := os.WriteFile(filepath.Join(d, "editable_manifest.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		if !hasVerifiedManagedManifest(d, snap) {
			t.Fatal("legacy verified for generation should still be recognized")
		}
		if got := hasVerifiedRetirableControls(d, snap); got != tc.want {
			t.Fatalf("sizing %s: got %v", tc.sizing, got)
		}
	}
}
