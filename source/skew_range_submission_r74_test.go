package main

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestR74SkewRangeTaskSerializesBothLimits(t *testing.T) {
	w := Workspace{Dir: t.TempDir()}
	s := &Snapshot{Project: "E:/ALD/current", Fingerprint: "mesh-fingerprint", Mesh: map[string]any{"Elements": 10055480}}
	task, err := w.Submit("mark_high_skew", s, &Plan{SkewMin: 0.90, SkewMax: 1.00}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if task.SkewMin != .9 || task.SkewMax != 1 || task.Phase != "mark_high_skew" {
		t.Fatalf("range lost: %+v", task)
	}
	raw, err := os.ReadFile(w.path("job.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["skew_min"] != 0.9 || m["skew_max"] != 1.0 {
		t.Fatalf("wrong JSON bounds: %s", string(raw))
	}
	if m["apply_global"] == true || m["overwrite"] == true {
		t.Fatalf("range marker must not mutate global sizing: %s", raw)
	}
	if m["expected_project"] != s.Project || m["geometry_fingerprint"] != s.Fingerprint {
		t.Fatalf("wrong project: %s", raw)
	}
}

func TestR74InvalidSkewRangesNeverCreateTaskLease(t *testing.T) {
	cases := []struct{ min, max float64 }{{1, 1}, {.96, .90}, {-.1, 1}, {.90, 1.01}, {math.NaN(), 1}, {.9, math.Inf(1)}}
	for _, tc := range cases {
		w := Workspace{Dir: t.TempDir()}
		snap := &Snapshot{Project: "E:/ALD/current", Fingerprint: "fp"}
		_, err := w.Submit("mark_high_skew", snap, &Plan{SkewMin: tc.min, SkewMax: tc.max}, "", false)
		if err == nil || !strings.Contains(err.Error(), "偏度范围") {
			t.Fatalf("should reject %v,%v: %v", tc.min, tc.max, err)
		}
		if _, err := os.Stat(w.taskLeasePath()); !os.IsNotExist(err) {
			t.Fatalf("invalid input created lease: %v", err)
		}
		if _, err := os.Stat(w.path("job.json")); !os.IsNotExist(err) {
			t.Fatalf("invalid input created job: %v", err)
		}
	}
}
