//go:build !windows

package main

func inspectMeshingProcess(pid int, startedFiletime string) (string, string) {
	return "unknown", "非 Windows 环境不检查 Meshing 进程"
}
