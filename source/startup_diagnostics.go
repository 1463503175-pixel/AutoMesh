package main

import (
	"time"
)

type startupDiagnostic struct {
	Status     string `json:"status"`
	Workspace  string `json:"workspace"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Error      string `json:"error,omitempty"`
}

// InitializeScriptsAndRecord never starts an ANSYS session, clears a lease,
// or silently overrides customized scripts. The diagnostic is best effort:
// a read-only account may fail to write it, but the original install error
// must still be returned unchanged to the UI.
func (w Workspace) InitializeScriptsAndRecord() error {
	start := time.Now()
	// Verified terminal results permit safe recovery after closing AutoMesh.
	// An unconfirmed job is NEVER cleared, even when the UI was restarted.
	recoverErr := w.ReconcileFinishedLease()
	err := recoverErr
	if err == nil {
		err = w.InstallAssets()
	}
	r := startupDiagnostic{
		Status: "ready", Workspace: w.Dir,
		StartedAt: start.Format(time.RFC3339Nano), FinishedAt: time.Now().Format(time.RFC3339Nano),
	}
	if err != nil {
		r.Status = "failed"
		r.Error = err.Error()
	}
	_ = writeJSON(w.path("startup_diagnostic.json"), r)
	return err
}
