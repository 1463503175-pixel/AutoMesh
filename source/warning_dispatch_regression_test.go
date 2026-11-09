package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Regression: in R6 the two quality-page buttons had ACT handlers but were
// omitted from Workspace.Submit, so the GUI returned "不支持该任务阶段" before
// any task reached Meshing. Check every direct GUI task, not only these two.
func TestEveryGUITaskCanBeSubmitted(t *testing.T) {
	gui, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	// Candidate publish is routed through a separate native review panel.
	reviewGUI, reviewErr := os.ReadFile("candidate_review_windows.go")
	if reviewErr != nil {
		t.Fatal(reviewErr)
	}
	gui = append(gui, reviewGUI...)
	// These are the actual dispatch sites for button actions.
	runCalls := regexp.MustCompile(`a\.run\("([a-z_]+)"\)`)
	taskCalls := regexp.MustCompile(`a\.taskRun\("([a-z_]+)"`)
	phases := map[string]bool{}
	for _, match := range runCalls.FindAllSubmatch(gui, -1) {
		phases[string(match[1])] = true
	}
	for _, match := range taskCalls.FindAllSubmatch(gui, -1) {
		phases[string(match[1])] = true
	}
	for _, phase := range []string{"warning_refine_preview", "warning_refine_publish"} {
		if !phases[phase] {
			t.Fatalf("GUI no longer calls %q", phase)
		}
	}
	for phase := range phases {
		phase := phase
		t.Run(phase, func(t *testing.T) {
			w := Workspace{Dir: t.TempDir()}
			snap := &Snapshot{Project: "E:/ALD/current", Fingerprint: "current-geometry", Mesh: map[string]any{"Elements": 9660873}}
			plan := &Plan{GlobalMM: 12, ExpectedProject: snap.Project, Fingerprint: snap.Fingerprint, SkewMin: 0.90, SkewMax: 1.00, QualityMin: 0.00, QualityMax: 0.10}
			var s *Snapshot = snap
			if phase == "inspect" || phase == "auto_onepass" {
				s = nil
			}
			export := ""
			if phase == "export" {
				export = "D:/Mesh/current.msh"
			}
			task, err := w.Submit(phase, s, plan, export, false)
			if err != nil {
				t.Fatalf("GUI phase %q rejected: %v", phase, err)
			}
			// Native deep distribution is an opt-in flag on the existing
			// read-only quality protocol, not a separate ACT phase.
			expectedPhase := phase
			if phase == "quality_check_deep" {
				expectedPhase = "quality_check"
			}
			if task.Phase != expectedPhase || task.DeepNativeSkewness != (phase == "quality_check_deep") {
				t.Fatalf("phase/opt-in flag mismatch: phase=%q deep=%v", task.Phase, task.DeepNativeSkewness)
			}
			if phase == "warning_refine_preview" || phase == "warning_refine_publish" {
				if task.ExpectedProject != snap.Project || task.Fingerprint != snap.Fingerprint {
					t.Fatalf("warning refinement lost model identity: %+v", task)
				}
				if task.ApplyGlobal || task.Overwrite || task.ExportPath != "" {
					t.Fatalf("warning refinement unexpectedly alters global/output settings: %+v", task)
				}
			}
		})
	}
}

func TestUnknownTaskStillRejectedBeforeTakingLease(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	snap := &Snapshot{Project: "E:/ALD/current", Fingerprint: "a"}
	_, err := w.Submit("not_a_real_phase", snap, nil, "", false)
	if err == nil || !strings.Contains(err.Error(), "不支持该任务阶段") {
		t.Fatalf("missing failure: %v", err)
	}
	if _, err := os.Stat(w.taskLeasePath()); !os.IsNotExist(err) {
		t.Fatalf("unexpected lock: %v", err)
	}
	if _, err := os.Stat(w.path("job.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected job: %v", err)
	}
}
