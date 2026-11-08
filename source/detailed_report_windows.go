//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Generate reports from the same audited version embedded in AutoMesh.exe.
// ShellExecuteW avoids a cmd.exe dependency and correctly handles Windows
// user-selected paths (including spaces and shell metacharacters).
// This action does not send any ANSYS command or modify the mesh.
func openReportPath(hwnd uintptr, path string) {
	sh := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	op := syscall.StringToUTF16Ptr("open")
	file := syscall.StringToUTF16Ptr(path)
	code, _, callErr := sh.Call(hwnd, uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	if code <= 32 {
		alert(fmt.Sprintf("报告无法打开（ShellExecuteW=%d）：%s；错误：%v", code, path, callErr))
	}
}
func (a *state) openDetailedReport() {
	// An already verified historical report is read-only and may be opened while
	// another task runs; never relabel it as the new in-flight task result.
	report := a.qualityResult
	if report == nil || a.qualityStale || a.snapshot == nil || a.qualityFingerprint == "" || a.qualityFingerprint != a.snapshot.Fingerprint {
		// Historical HTML is allowed in read-only mode after restarting AM.
		// It is NOT claimed to represent the presently loaded Meshing grid.
		historical := a.ws.path("quality_report.html")
		if _, e := os.Stat(historical); e != nil {
			alert("当前没有经实时核验的网格质量；历史报告也不存在。请只读检查当前网格。")
			return
		}
		if !confirm("将打开上一次保存的历史质量报告。它可能不对应当前打开的网格，不能用于判定当前质量。确定只读查看？") {
			return
		}
		openReportPath(a.window, historical)
		return
	}
	path, err := a.ws.SaveHTMLReport(a.snapshot, *report)
	if err != nil {
		alert("生成质量报告失败：" + err.Error())
		return
	}
	if _, err = os.Stat(path); err != nil {
		alert("报告文件不可读取：" + fmt.Sprint(err))
		return
	}
	openReportPath(a.window, path)
}

// Open the plan produced by the last reviewed read-only worker job.
// Never substitute a file from another model or prior session.
func (a *state) openWarningPlanReport() {
	if a.snapshot == nil {
		archive := a.ws.path("warning_refinement_plan.html")
		if _, e := os.Stat(archive); e != nil {
			alert("当前没有已验证的工程快照，也没有已保存的诊断报告；请先扫描并标记警告。 ")
			return
		}
		if confirm("将打开历史警告区诊断报告，仅供参考，不能直接发布到当前 Meshing。确定？") {
			openReportPath(a.window, archive)
		}
		return
	}
	plan, err := a.ws.ReadReviewedWarningDiagnostic(a.snapshot)
	if err != nil {
		// After marking, the user can inspect the selected element IDs even
		// before requesting a warning-refinement preview. This is READ ONLY.
		if markerPath, markErr := a.ws.SaveSkewMarkerHTML(a.snapshot); markErr == nil {
			openReportPath(a.window, markerPath)
			return
		}
		alert("没有可查看的已完成方案：" + err.Error() + "\n当前执行中的任务不会被当作成功方案；请先标记偏度单元，或生成局部细化方案。")
		return
	}
	// View-only: allowed even when another job is pending. Publication remains
	// blocked by the exclusive lease until the remote state is confirmed.
	path, err := a.ws.SaveWarningPlanHTML(a.snapshot, plan.PlanID)
	if err != nil {
		alert("无法打开已审核的当前方案：" + err.Error())
		return
	}
	openReportPath(a.window, path)
}
