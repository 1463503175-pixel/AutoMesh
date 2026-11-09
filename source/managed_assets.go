package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// managedScriptNames is deliberately closed: only these seven embedded
// runtime scripts are upgraded. Reports, job receipts, plans and logs in Data
// are never inspected for replacement or removed during a version upgrade.
var managedScriptNames = []string{
	"meshing_worker.py", "workbench_bridge.py", "wb_cli.py",
	"geometry_rules.py", "warning_refinement.py", "wb_server_bootstrap.py",
	"start_wb_server_uia.ps1",
}

const managedAssetReceiptFilename = "installed_assets_receipt.json"

// An installer receipt records exact bytes installed by the preceding
// AutoMesh release; it is NOT an authorization to replace arbitrary scripts.
// If a user edits even one installed script, that file no longer matches its
// receipt and is protected against silent replacement.
type managedAssetReceipt struct {
	SchemaVersion int               `json:"schema_version"`
	ManagedBy     string            `json:"managed_by"`
	Hashes        map[string]string `json:"sha256"`
}

func assetSHA(b []byte) string {
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}

func (r *managedAssetReceipt) valid() bool {
	if r == nil || r.SchemaVersion != 1 || r.ManagedBy != "AutoMesh" || len(r.Hashes) != len(managedScriptNames) {
		return false
	}
	for _, name := range managedScriptNames {
		h, ok := r.Hashes[name]
		if !ok || len(h) != 64 || strings.ToLower(h) != h {
			return false
		}
		if _, err := hex.DecodeString(h); err != nil {
			return false
		}
	}
	return true
}

func (r *managedAssetReceipt) owns(name string, actual []byte) bool {
	return r.valid() && r.Hashes[name] == assetSHA(actual)
}

func (w Workspace) readInstalledAssetReceipt() *managedAssetReceipt {
	b, err := os.ReadFile(w.path(managedAssetReceiptFilename))
	if err != nil || len(b) > 16*1024 {
		return nil
	}
	var r managedAssetReceipt
	if json.Unmarshal(b, &r) != nil || !r.valid() {
		return nil
	}
	return &r
}

// Save only after full installation; verify on-disk bytes against embedded
// assets so the next release cannot mistake a partial/mixed install as owned.
func (w Workspace) writeInstalledAssetReceipt() error {
	r := managedAssetReceipt{SchemaVersion: 1, ManagedBy: "AutoMesh", Hashes: make(map[string]string, len(managedScriptNames))}
	for _, name := range managedScriptNames {
		want, err := embedded.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(w.path(name))
		if err != nil {
			return err
		}
		if assetSHA(got) != assetSHA(want) {
			return fmt.Errorf("runtime script %s changed during upgrade: refusing to record receipt", name)
		}
		r.Hashes[name] = assetSHA(got)
	}
	if previous := w.readInstalledAssetReceipt(); previous != nil {
		unchanged := true
		for name, expected := range r.Hashes {
			if previous.Hashes[name] != expected {
				unchanged = false
				break
			}
		}
		if unchanged {
			return nil // No repeated manifest writes on ordinary tasks or startup.
		}
	}
	return writeJSON(w.path(managedAssetReceiptFilename), r)
}
