package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestR801RecoversTerminalReceiptWriteFailureWithoutInventingSuccess(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	jobID := strings.Repeat("a", 32)
	lease := Task{JobID: jobID, Phase: "warning_refine_preview"}
	if err := writeJSON(w.taskLeasePath(), lease); err != nil {
		t.Fatal(err)
	}
	progress := ActProgress{
		JobID: jobID, Stage: "终态结果全部写入失败",
		Detail:    "job receipt and last_result replace failed",
		Timestamp: float64(time.Now().Add(-15*time.Second).UnixNano()) / 1e9,
	}
	if err := writeJSON(w.path("job_progress_"+jobID+".json"), progress); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); !os.IsNotExist(err) {
		t.Fatalf("terminal write-failure marker should release old lease, stat err=%v", err)
	}
	result, ok := w.terminalResultForJob(jobID)
	if !ok {
		t.Fatal("expected a durable recovered terminal error receipt")
	}
	if result.Status != "error" {
		t.Fatalf("must never invent success; got status %q", result.Status)
	}
	if !strings.Contains(result.Error, "不能据此推断操作成功或失败") {
		t.Fatalf("recovered result must preserve uncertainty: %q", result.Error)
	}
}

func TestR801DoesNotRecoverFreshReceiptWriteFailureMarker(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	jobID := strings.Repeat("b", 32)
	lease := Task{JobID: jobID, Phase: "warning_refine_preview"}
	if err := writeJSON(w.taskLeasePath(), lease); err != nil {
		t.Fatal(err)
	}
	progress := ActProgress{
		JobID: jobID, Stage: "终态结果全部写入失败",
		Timestamp: float64(time.Now().UnixNano()) / 1e9,
	}
	if err := writeJSON(w.path("job_progress_"+jobID+".json"), progress); err != nil {
		t.Fatal(err)
	}
	if err := w.ReconcileFinishedLease(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.taskLeasePath()); err != nil {
		t.Fatalf("fresh marker must keep lease until the 10-second safety delay: %v", err)
	}
}
