package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// A persisted manifest is a hint only. The ACT worker independently re-reads
// all named scopes and control identities before modifying Meshing.
// Return false for old/unverified/manual-only manifests.
func hasVerifiedManagedManifest(workspace string, snapshot *Snapshot) bool {
	return inspectManagedManifest(workspace, snapshot, false)
}

// Only native AutoMesh_NS controls can be auto-suppressed by the current ACT
// worker. Older AM_* AutoMesh manifests still permit reviewed generation, but
// must not unlock an all-disabled native republish task.
func hasVerifiedRetirableControls(workspace string, snapshot *Snapshot) bool {
	return inspectManagedManifest(workspace, snapshot, true)
}

func expectedManagedSizingName(name string) string {
	ascii := strings.Builder{}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			ascii.WriteRune(r)
		} else {
			ascii.WriteRune('_')
		}
	}
	prefix := strings.Trim(ascii.String(), "_")
	if len(prefix) > 22 {
		prefix = prefix[:22]
	}
	if prefix == "" {
		prefix = "Group"
	}
	sum := sha1.Sum([]byte(name))
	return "AutoMesh_NS_" + prefix + "_" + hex.EncodeToString(sum[:])[:10]
}

func inspectManagedManifest(workspace string, snapshot *Snapshot, onlyRetirable bool) bool {
	if snapshot == nil || snapshot.Project == "" || snapshot.Fingerprint == "" {
		return false
	}
	b, err := os.ReadFile(filepath.Join(workspace, "editable_manifest.json"))
	if err != nil || len(b) == 0 || len(b) > 2<<20 {
		return false
	}
	var m struct {
		Status      string `json:"status"`
		Project     string `json:"project"`
		Fingerprint string `json:"fingerprint"`
		Prepared    []struct {
			NS     string `json:"ns"`
			Sizing string `json:"sizing"`
		} `json:"prepared"`
		Retired []struct {
			NS     string `json:"ns"`
			Sizing string `json:"sizing"`
		} `json:"retired_managed"`
	}
	if json.Unmarshal(b, &m) != nil || m.Status != "ready" || m.Project != snapshot.Project || m.Fingerprint != snapshot.Fingerprint {
		return false
	}
	for _, p := range m.Prepared {
		if p.NS != "" && p.Sizing != "" && (!onlyRetirable || p.Sizing == expectedManagedSizingName(p.NS)) {
			return true
		}
	}
	for _, p := range m.Retired {
		if p.NS != "" && p.Sizing != "" && (!onlyRetirable || p.Sizing == expectedManagedSizingName(p.NS)) {
			return true
		}
	}
	return false
}
