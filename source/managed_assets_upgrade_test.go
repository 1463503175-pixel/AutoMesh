package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR81KnownReleaseMigratesWithoutDeletingData(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	// Real SHA-256s from the previous R8.1 candidate archives, not guessed
	// aliases. The standalone guard is checked by earlier regression tests.
	if !knownLegacyWorkerHashes["db33a9c1641a741249010435515937a6828734eede6c7622eb9e7c967f3e4ac1"] ||
		!knownLegacyWorkerHashes["b3ae587045e191ea2eeb45a20b02434c2e146497b1a4f8c84a81adfc6db5dc9b"] ||
		!knownLegacyAssetHashes["warning_refinement.py"]["8c8a393f8dbd30828c7007d544476b5cf5d16ac8a81a73f6d4a8216d08d0ec28"] {
		t.Fatal("verified R8.1 script upgrade compatibility missing")
	}
	if err := os.WriteFile(w.path("quality_history.json"), []byte("USER_HISTORY_NOT_TO_TOUCH"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.InstallAssets(); err != nil {
		t.Fatal(err)
	}
	receipt := w.readInstalledAssetReceipt()
	if receipt == nil || !receipt.valid() {
		t.Fatal("installation did not persist managed script hashes")
	}
	previous := receipt.Hashes["meshing_worker.py"]
	if previous != assetSHA(mustReadAsset(t, w.path("meshing_worker.py"))) {
		t.Fatal("wrong installed worker identity")
	}
	if err := w.InstallAssets(); err != nil {
		t.Fatal("second install not idempotent:", err)
	}
	b, err := os.ReadFile(w.path("quality_history.json"))
	if err != nil || string(b) != "USER_HISTORY_NOT_TO_TOUCH" {
		t.Fatal("existing reports were modified")
	}
}

func mustReadAsset(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestReceiptAuthorizesOnlyExactPreviouslyInstalledBytes(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	if err := w.InstallAssets(); err != nil {
		t.Fatal(err)
	}
	original := mustReadAsset(t, w.path("meshing_worker.py"))
	receipt := w.readInstalledAssetReceipt()
	if !receipt.owns("meshing_worker.py", original) {
		t.Fatal("valid installed worker not recognized")
	}
	if receipt.owns("meshing_worker.py", append(original, []byte("\n# CUSTOM")...)) {
		t.Fatal("user customization accepted")
	}
	if receipt.owns("unknown.py", original) {
		t.Fatal("unmanaged script accepted")
	}

	// A script that differs from the signed-in receipt is refused even when
	// the receipt itself is present and other managed assets are unchanged.
	if err := os.WriteFile(w.path("meshing_worker.py"), []byte("# CUSTOM USER WORKER"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.InstallAssets(); err == nil || !strings.Contains(err.Error(), "unrecognized/customized") {
		t.Fatalf("custom worker overwritten: %v", err)
	}
	if string(mustReadAsset(t, w.path("meshing_worker.py"))) != "# CUSTOM USER WORKER" {
		t.Fatal("custom source changed")
	}
}

func TestOwnedPreviousVersionAndBackupArePreserved(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	if err := w.InstallAssets(); err != nil {
		t.Fatal(err)
	}
	old := []byte("# prior managed release; deliberately different from current\n")
	receipt := w.readInstalledAssetReceipt()
	receipt.Hashes["meshing_worker.py"] = assetSHA(old)
	if err := writeJSON(w.path(managedAssetReceiptFilename), receipt); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.path("meshing_worker.py"), old, 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.InstallAssets(); err != nil {
		t.Fatal("cannot upgrade receipt-owned prior release:", err)
	}
	installed, _ := embedded.ReadFile("assets/meshing_worker.py")
	if string(mustReadAsset(t, w.path("meshing_worker.py"))) != string(installed) {
		t.Fatal("new worker not installed")
	}
	if !w.readInstalledAssetReceipt().owns("meshing_worker.py", installed) {
		t.Fatal("new receipt does not match current release")
	}
	backup := w.path("meshing_worker.py") + ".before-" + assetSHA(old)[:16] + ".bak"
	if string(mustReadAsset(t, backup)) != string(old) {
		t.Fatal("original not backed up")
	}
}

func TestUnconfirmedLeaseBlocksManagedUpgradeWithoutChangingAnything(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	if err := w.InstallAssets(); err != nil {
		t.Fatal(err)
	}
	old := []byte("# old release\n")
	r := w.readInstalledAssetReceipt()
	r.Hashes["meshing_worker.py"] = assetSHA(old)
	if err := writeJSON(w.path(managedAssetReceiptFilename), r); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.path("meshing_worker.py"), old, 0600); err != nil {
		t.Fatal(err)
	}
	lease := Task{JobID: strings.Repeat("a", 32), Phase: "quality_check"}
	if err := writeJSON(w.taskLeasePath(), lease); err != nil {
		t.Fatal(err)
	}
	if err := w.InstallAssets(); err == nil || !strings.Contains(err.Error(), "未确认结束") {
		t.Fatalf("active job not protected: %v", err)
	}
	if string(mustReadAsset(t, w.path("meshing_worker.py"))) != string(old) {
		t.Fatal("worker changed under unconfirmed job")
	}
	if !r.owns("meshing_worker.py", mustReadAsset(t, w.path("meshing_worker.py"))) {
		t.Fatal("receipt changed under active job")
	}
	if _, err := os.Stat(filepath.Join(w.Dir, "quality_history.json")); !os.IsNotExist(err) {
		t.Fatal("unexpected history mutation")
	}
}

// Optional end-to-end regression with an actual prior release archive. Set
// AUTOMESH_TEST_PREVIOUS_RELEASE_ZIP when a historical release ZIP is present.
// The default source package remains standalone and runnable without it.
func TestUpgradeFromActualPreviousRCArchive(t *testing.T) {
	oldRelease := os.Getenv("AUTOMESH_TEST_PREVIOUS_RELEASE_ZIP")
	if oldRelease == "" {
		t.Skip("prior release ZIP not supplied")
	}
	z, err := zip.OpenReader(oldRelease)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	w := Workspace{Dir: t.TempDir()}
	history := []byte("existing quality snapshots and prior task receipts")
	if err := os.WriteFile(w.path("quality_history.json"), history, 0600); err != nil {
		t.Fatal(err)
	}
	loaded := map[string]bool{}
	for _, f := range z.File {
		for _, name := range managedScriptNames {
			if f.Name != "完整源码/assets/"+name {
				continue
			}
			r, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			prior, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(w.path(name), prior, 0600); err != nil {
				t.Fatal(err)
			}
			loaded[name] = true
		}
	}
	if len(loaded) != len(managedScriptNames) {
		t.Fatal(fmt.Sprintf("only %d old assets loaded", len(loaded)))
	}
	if err := w.InitializeScriptsAndRecord(); err != nil {
		t.Fatal("authentic previous release should migrate without deleting Data:", err)
	}
	for _, name := range managedScriptNames {
		got := mustReadAsset(t, w.path(name))
		expected, _ := embedded.ReadFile("assets/" + name)
		if assetSHA(got) != assetSHA(expected) {
			t.Fatalf("old asset %s not upgraded", name)
		}
	}
	if string(mustReadAsset(t, w.path("quality_history.json"))) != string(history) {
		t.Fatal("historical records modified")
	}
	if w.readInstalledAssetReceipt() == nil {
		t.Fatal("new install receipt missing")
	}
}
