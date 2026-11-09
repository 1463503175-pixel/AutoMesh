package main

import (
	"strings"
	"testing"
	"time"
)

func TestTaskStatusRefreshDue(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	if !taskStatusRefreshDue(time.Time{}, now) {
		t.Fatal("first task status read must be immediate")
	}
	if taskStatusRefreshDue(now, now.Add(time.Second)) {
		t.Fatal("status refresh should be throttled within the two-second interval")
	}
	if !taskStatusRefreshDue(now, now.Add(2*time.Second)) {
		t.Fatal("status refresh must become due after the interval")
	}
	if !taskStatusRefreshDue(now, now.Add(-time.Second)) {
		t.Fatal("clock rollback should force a refresh rather than freeze stale status")
	}
}

func TestTaskStatusReportsStaleHeartbeatWithoutClaimingProcessExit(t *testing.T) {
	workspace := Workspace{Dir: t.TempDir()}
	jobID := "job-heartbeat-test"
	lease := Task{JobID: jobID, Phase: "inspect", MeshSystem: "FFF", SubmittedAt: time.Now().Add(-3 * time.Minute).Format(time.RFC3339)}
	if err := writeJSON(workspace.taskLeasePath(), lease); err != nil {
		t.Fatalf("write lease fixture: %v", err)
	}
	progress := ActProgress{JobID: jobID, Stage: "读取几何", Detail: "等待回执", Timestamp: float64(time.Now().Add(-2 * time.Minute).Unix())}
	if err := writeJSON(workspace.path("job_progress_"+jobID+".json"), progress); err != nil {
		t.Fatalf("write progress fixture: %v", err)
	}
	status := workspace.taskStatus()
	if !strings.Contains(status, "心跳超时") {
		t.Fatalf("stale heartbeat should be explicit: %s", status)
	}
	if !strings.Contains(status, "不代表 Meshing 已停止") || !strings.Contains(status, "任务锁继续保留") {
		t.Fatalf("heartbeat timeout must not be interpreted as process exit or lease release: %s", status)
	}
}
