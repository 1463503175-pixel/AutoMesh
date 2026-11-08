//go:build !windows

package main

import (
	"encoding/json"
	"os"
)

// Development-only layout exporter for faithful, deterministic preview rendering.
// This file is excluded from the Windows executable.
func main() {
	widths := []int{1100, 1240, 1366, 1600}
	out := make(map[int]map[string]uiBox)
	for _, w := range widths {
		out[w] = plannedControls(w)
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
}
