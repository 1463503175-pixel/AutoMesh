package main

import (
	"os"
	"strings"
	"testing"
)

func TestFinalGlobalAuditNeverSubstitutesReference(t *testing.T) {
	snap := &Snapshot{Project: "P", Fingerprint: strings.Repeat("a", 40), Mesh: map[string]any{
		"ElementSize": "0.0 [mm]", "MaximumSize": "24.0 [mm]", "GrowthRate": "1.3", "CaptureCurvature": "False", "SolverPreference": "Fluent",
	}}
	text := globalReviewText(snap, 12, false)
	if !strings.Contains(text, "自动/未解析") || !strings.Contains(text, "AutoMesh 参考尺寸：12 mm") || !strings.Contains(text, "不修改 Meshing 全局尺寸") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "来源：上一次只读扫描") || !strings.Contains(text, "邻近捕捉：未获取") {
		t.Fatal(text)
	}
}
func TestFinalTaskStatusReadOnlyIdentityAndNoLeaseMutation(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	no := w.taskStatus()
	if !strings.Contains(no, "没有待确认") {
		t.Fatal(no)
	}
	id := strings.Repeat("b", 32)
	if err := writeJSON(w.taskLeasePath(), Task{JobID: id, Phase: "generate_reviewed"}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(w.path("wb_dispatch_"+id+".json"), DispatchReceipt{JobID: id, Status: "wb_target_verified"}); err != nil {
		t.Fatal(err)
	}
	st := w.taskStatus()
	if !strings.Contains(st, "wb_target_verified") || !strings.Contains(st, "Meshing 阶段：未获确认") {
		t.Fatal(st)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatal("read-only check deleted live lock")
	}
	state, r := remoteJobObservation(w, id)
	if state != "wb_verified" || r != nil {
		t.Fatalf("%s %v", state, r)
	}
	if err := writeJSON(w.path("last_result.json"), Result{JobID: id, Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	state, r = remoteJobObservation(w, id)
	if state != "act_terminal" || r == nil {
		t.Fatalf("%s %v", state, r)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatal("observation must not release lock")
	}
}
func TestFinalVerifiedUpgradeBridgeAndWorker(t *testing.T) {
	// This source-independent constant also checks the intended verified digest.
	if !knownLegacyWorkerHashes["8188e0608d378da97d9d1e0876cf79108fe9dce11dc8ceb14fa2ee83"] {
		t.Fatal("R5.3 worker blocked")
	}
	if err := checkGeneratedAssetReplacement("workbench_bridge.py", []byte("manual user edit"), []byte("new")); err == nil {
		t.Fatal("custom bridge overwritten")
	}
	if err := checkGeneratedAssetReplacement("meshing_worker.py", []byte("manual user edit"), []byte("new")); err == nil {
		t.Fatal("custom worker overwritten")
	}
}
func TestFinalReportsAndFineActionsHaveNoOverlapAtMinWidth(t *testing.T) {
	for _, width := range []int{1100, 1120, 1280, 1366, 1920} {
		boxes := plannedControls(width)
		for _, keys := range [][]string{
			{"qualitycheck", "htmlreport", "diagnose", "mark", "markwarning", "warningplan", "warningview", "warningpublish", "taskstatus"},
			{"finenames", "fineprepare", "finegenerate", "fineglobalreview"},
		} {
			for i := 0; i < len(keys); i++ {
				for j := i + 1; j < len(keys); j++ {
					if boxes[keys[i]].intersects(boxes[keys[j]]) {
						t.Fatalf("width %d: %s overlaps %s", width, keys[i], keys[j])
					}
				}
			}
		}
		for _, name := range []string{"fineglobalreview", "taskstatus", "finegenerate", "markwarning"} {
			b := boxes[name]
			if b.Y+b.H > 690 || b.X+b.W > width {
				t.Fatalf("width %d: %s clipped %+v", width, name, b)
			}
		}
	}
}
