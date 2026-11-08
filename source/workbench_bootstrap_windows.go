//go:build windows

package main

import (
	"fmt"
	"time"
)

type serverBootstrapResult struct {
	Status         string `json:"status"`
	Detail         string `json:"detail"`
	ProcessID      int    `json:"process_id"`
	CandidatePorts []int  `json:"candidate_ports"`
}

// discoverExistingServerPorts uses Windows' TCP LISTENING inventory. Unlike
// the old hard-coded 10000..11200 scan it finds arbitrary StartServer ports.
// It never invokes UI Automation, starts a server, or modifies the project.
func (a *state) discoverExistingServerPorts(preferred int) ([]int, string, error) {
	a.postProgress("I|查找已启动的 Workbench 端口|读取 Windows 全端口监听表和关联进程，暂不执行 StartServer()")
	buf, err := runPython(13*time.Second, a.ws.path("wb_server_bootstrap.py"), "--mode", "discover", "--preferred", fmt.Sprint(preferred))
	if len(buf) == 0 {
		return nil, "", fmt.Errorf("端口枚举未产生输出: %v", err)
	}
	var r struct {
		Status         string `json:"status"`
		Detail         string `json:"detail"`
		CandidatePorts []int  `json:"candidate_ports"`
		ProcessPorts   []int  `json:"process_ports"`
		ListenerCount  int    `json:"listener_count"`
	}
	if e := parseResponse(buf, &r); e != nil {
		return nil, "", e
	}
	if r.Status != "discovered" {
		return nil, r.Detail, fmt.Errorf("端口枚举失败: %s", r.Detail)
	}
	if err != nil {
		return nil, r.Detail, err
	}
	return r.CandidatePorts, fmt.Sprintf("检测到 %d 个本机监听端口，%d 个直接关联 Workbench；均需验证协议", r.ListenerCount, len(r.ProcessPorts)), nil
}

func (a *state) probeExistingWorkbench() (serverBootstrapResult, error) {
	a.postProgress("I|确认现有 Workbench 进程|只读查找当前工作台监听端口")
	buf, err := runPython(21*time.Second, a.ws.path("wb_server_bootstrap.py"), "--mode", "probe")
	if err != nil && len(buf) == 0 {
		return serverBootstrapResult{}, fmt.Errorf("无法识别当前 Workbench: %v", err)
	}
	var out serverBootstrapResult
	if e := parseResponse(buf, &out); e != nil {
		return out, e
	}
	if out.Status != "identified" {
		return out, fmt.Errorf("Workbench 主窗口识别不唯一: %s", out.Detail)
	}
	if err != nil {
		return out, err
	}
	return out, nil
}

// bootstrapExistingWorkbench contacts only an existing, uniquely identified
// Workbench GUI command window. It never launches Workbench. The returned ports
// are untrusted until wbRun("list") has verified a real project Mesh system.
func (a *state) bootstrapExistingWorkbench() (serverBootstrapResult, error) {
	a.postProgress("I|尝试启动现有 Workbench 脚本服务|通过 Windows 可访问性控件执行 StartServer()，不使用截图定位")
	buf, err := runPython(30*time.Second, a.ws.path("wb_server_bootstrap.py"), "--mode", "start")
	if len(buf) == 0 {
		return serverBootstrapResult{}, fmt.Errorf("脚本服务自动启动器未返回状态: %v", err)
	}
	var out serverBootstrapResult
	if e := parseResponse(buf, &out); e != nil {
		return out, fmt.Errorf("自动启动器响应格式错误: %w", e)
	}
	if out.Status != "submitted" {
		return out, fmt.Errorf("未能安全执行 StartServer(): %s", out.Detail)
	}
	if err != nil {
		return out, fmt.Errorf("启动命令提交结果不确定: %v；禁止自动重试", err)
	}
	return out, nil
}

func uniqueCandidatePorts(primary []int, second []int, limit int) []int {
	visited := map[int]bool{}
	result := []int{}
	for _, p := range append(append([]int{}, primary...), second...) {
		if p <= 0 || p > 65535 || visited[p] {
			continue
		}
		visited[p] = true
		result = append(result, p)
		if len(result) >= limit {
			break
		}
	}
	return result
}
