//go:build windows

package main

import (
	"fmt"
	"strconv"
	"syscall"
)

// Probe a PID recorded by ACT itself, not a guessed process from the taskbar.
// A mismatched start timestamp means the PID has been recycled.
func inspectMeshingProcess(pid int, startedFiletime string) (string, string) {
	if pid <= 0 || startedFiletime == "" {
		return "unknown", "Meshing 未提供可靠进程身份"
	}
	expected, err := strconv.ParseUint(startedFiletime, 10, 64)
	if err != nil || expected == 0 {
		return "unknown", "进程启动时间无效"
	}
	h, err := syscall.OpenProcess(0x1000, false, uint32(pid)) // PROCESS_QUERY_LIMITED_INFORMATION
	if err != nil {
		if err == syscall.Errno(87) {
			return "exited", "系统中已不存在原 PID"
		}
		return "unknown", fmt.Sprintf("无法查询 PID（%v）", err)
	}
	defer syscall.CloseHandle(h)
	var creation, exit, kernel, user syscall.Filetime
	if e := syscall.GetProcessTimes(h, &creation, &exit, &kernel, &user); e != nil {
		return "unknown", fmt.Sprintf("无法读取进程启动时间（%v）", e)
	}
	got := uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)
	if got != expected {
		return "exited", "原 PID 已被新进程复用"
	}
	state, e := syscall.WaitForSingleObject(h, 0) // read-only, never kills process
	if e != nil {
		return "unknown", fmt.Sprintf("无法查询进程退出状态（%v）", e)
	}
	switch state {
	case 0:
		return "exited", "原 Meshing 进程已经终止"
	case 258:
		return "running", "原 Meshing 进程仍存活（不代表 ACT 没有卡住）"
	default:
		return "unknown", fmt.Sprintf("无法确定状态（%d）", state)
	}
}
