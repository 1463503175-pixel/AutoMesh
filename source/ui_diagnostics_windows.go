//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The log is an ordinary locally readable UTF-8 text file, opened in Notepad.
// It does not run ANSYS, delete a lease, or attempt to infer a missing result.
func (a *state) openTaskLog() {
	var lines []string
	lines = append(lines, "AutoMesh 任务与错误详情", "时间: "+time.Now().Format(time.RFC3339), "说明: 此日志是界面诊断，不代表 Meshing 操作一定成功或失败。", "")
	lines = append(lines, "界面状态: "+a.status)
	if a.activePhase != "" {
		lines = append(lines, "当前任务: "+a.activePhase)
	}
	if a.last != nil {
		lines = append(lines, fmt.Sprintf("最近回执: phase=%s, status=%s, job=%s", a.last.Phase, a.last.Status, a.last.JobID))
	}
	if a.errorText != "" {
		lines = append(lines, "", "错误原文:", a.errorText)
	}
	lines = append(lines, "", "近期事件:")
	lines = append(lines, a.history...)
	if t, pending, e := a.ws.UnconfirmedActiveTask(); e == nil && pending {
		lines = append(lines, fmt.Sprintf("待核实任务: phase=%s, job=%s", t.Phase, t.JobID))
	} else if e != nil {
		lines = append(lines, "任务占用状态无法核实: "+e.Error())
	}
	path := filepath.Join(a.ws.Dir, "AutoMesh_UI_Diagnostics.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0600); err != nil {
		alert("写入任务诊断失败: " + err.Error())
		return
	}
	if err := exec.Command("notepad.exe", path).Start(); err != nil {
		notice("诊断文件已保存，无法启动记事本，请手动打开：" + path)
	}
}
