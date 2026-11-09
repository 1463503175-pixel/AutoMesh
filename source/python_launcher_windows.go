//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type pythonCandidate struct {
	exe    string
	prefix []string
}

var pythonProbeOnce sync.Once
var pythonSelected pythonCandidate
var pythonProbeErr error

// Prefer a working runtime, not merely a launcher name. On Windows the `py`
// alias may exist without the selected Python or ansys-workbench-core package.
func choosePythonCandidate() (pythonCandidate, error) {
	pythonProbeOnce.Do(func() { pythonSelected, pythonProbeErr = probePythonCandidate() })
	return pythonSelected, pythonProbeErr
}
func probePythonCandidate() (pythonCandidate, error) {
	candidates := []pythonCandidate{}
	if exe, err := exec.LookPath("py"); err == nil {
		for _, version := range []string{"-3.11", "-3.12", "-3.10", "-3"} {
			candidates = append(candidates, pythonCandidate{exe: exe, prefix: []string{version}})
		}
	}
	if exe, err := exec.LookPath("python"); err == nil {
		candidates = append(candidates, pythonCandidate{exe: exe})
	}
	bases := []string{`C:\Python311`, `C:\Python312`}
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		bases = append(bases, filepath.Join(v, "Programs", "Python"))
	}
	if v := os.Getenv("ProgramFiles"); v != "" {
		bases = append(bases, filepath.Join(v, "Python"))
	}
	for _, base := range bases {
		if base == "" {
			continue
		}
		paths := []string{filepath.Join(base, "python.exe")}
		directories, _ := filepath.Glob(filepath.Join(base, "Python3*"))
		sort.Sort(sort.Reverse(sort.StringSlice(directories)))
		for _, dir := range directories {
			paths = append(paths, filepath.Join(dir, "python.exe"))
		}
		for _, path := range paths {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				candidates = append(candidates, pythonCandidate{exe: path})
			}
		}
	}
	unique := map[string]bool{}
	tried := []string{}
	for _, candidate := range candidates {
		key := strings.ToLower(candidate.exe + " " + strings.Join(candidate.prefix, " "))
		if unique[key] {
			continue
		}
		unique[key] = true
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		args := append(append([]string{}, candidate.prefix...), "-c", "import sys; import ansys.workbench.core; print('AUTOMESH_PYTHON_READY')")
		cmd := exec.CommandContext(ctx, candidate.exe, args...)
		// This probe runs during GUI startup. Without CREATE_NO_WINDOW, a
		// console-subsystem python.exe may briefly open its own terminal.
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		out, err := cmd.CombinedOutput()
		cancel()
		if err == nil && strings.Contains(string(out), "AUTOMESH_PYTHON_READY") {
			return candidate, nil
		}
		if len(tried) < 5 {
			tried = append(tried, fmt.Sprintf("%s %s (%s)", candidate.exe, strings.Join(candidate.prefix, " "), shorten(strings.TrimSpace(string(out)), 100)))
		}
	}
	if len(candidates) == 0 {
		return pythonCandidate{}, errors.New("未找到 Python 3 可执行程序；已检查 PATH、py launcher 和用户目录下的常见安装位置；请安装 Python 3.11 并安装 ansys-workbench-core")
	}
	return pythonCandidate{}, fmt.Errorf("找到 Python 但无法导入 ansys.workbench.core；尝试结果：%s。请在对应 Python 3.11 环境安装 ansys-workbench-core", strings.Join(tried, "; "))
}
