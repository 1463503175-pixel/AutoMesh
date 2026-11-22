//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const wmRestoreMain = 0x8000 + 81
const wmTrayEvent = 0x8000 + 82
const wmWorkerError = 0x8000 + 83

var trayPresent bool
var trayIcon uintptr
var taskbarCreated = uint32(call(usr.NewProc("RegisterWindowMessageW"), ptr("TaskbarCreated")))
var shellNotify = syscall.NewLazyDLL("shell32.dll").NewProc("Shell_NotifyIconW")
var eventMu sync.Mutex
var workerErrors = make(chan string, 16)

type notifyIconData struct {
	Size                uint32
	Window              uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Timeout             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                [16]byte
	BalloonIcon         uintptr
}

func lifecycleEvent(event string, detail any) {
	if ui.ws.Dir == "" {
		return
	}
	eventMu.Lock()
	defer eventMu.Unlock()
	b, e := json.Marshal(map[string]any{"time": time.Now().Format(time.RFC3339Nano), "pid": os.Getpid(), "event": event, "detail": detail})
	if e != nil {
		return
	}
	f, e := os.OpenFile(filepath.Join(ui.ws.Dir, "lifecycle.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
func restoreMainWindow(hwnd uintptr) {
	call(show, hwnd, 9) // SW_RESTORE also reveals a hidden window.
	call(usr.NewProc("SetForegroundWindow"), hwnd)
	lifecycleEvent("window_restore", nil)
}
func setTray(hwnd uintptr, add bool) bool {
	n := notifyIconData{Window: hwnd, ID: 1, Flags: 7, Callback: wmTrayEvent, Icon: trayIcon}
	n.Size = uint32(unsafe.Sizeof(n))
	tip := syscall.StringToUTF16("AutoMesh R8.1.2 - click to restore")
	copy(n.Tip[:], tip)
	operation := uintptr(2)
	if add {
		operation = 0
	}
	ok := call(shellNotify, operation, uintptr(unsafe.Pointer(&n))) != 0
	if ok {
		trayPresent = add
	}
	return ok
}
func hideToTray(hwnd uintptr) {
	if trayPresent || setTray(hwnd, true) {
		call(show, hwnd, 0)
		lifecycleEvent("window_hide", nil)
	} else {
		restoreMainWindow(hwnd)
		notice("无法创建托盘图标，主窗口保持可见。")
	}
}
func restoreExistingInstance() bool {
	find := usr.NewProc("FindWindowW")
	for i := 0; i < 50; i++ {
		hwnd := call(find, ptr("AutoMesh2024R2Modern"), 0)
		if hwnd != 0 {
			return call(post, hwnd, wmRestoreMain, 0, 0) != 0
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
func lifecycleMessage(hwnd uintptr, msg uint32, w, l uintptr) bool {
	switch msg {
	case wmRestoreMain:
		restoreMainWindow(hwnd)
		return true
	case wmTrayEvent:
		if l == 0x202 || l == 0x203 || l == 0x205 {
			restoreMainWindow(hwnd)
		}
		return true
	case WM_SIZE:
		if w == 1 {
			hideToTray(hwnd)
			return true
		}
	case 0x0010: // WM_CLOSE
		if ui.busy || ui.initRunning {
			answer := call(usr.NewProc("MessageBoxW"), hwnd, ptr("后台任务仍在运行。隐藏到托盘继续执行？取消任务请先恢复窗口并使用停止按钮。"), ptr("AutoMesh"), 0x24)
			if answer == 6 {
				hideToTray(hwnd)
			}
			return true
		}
		answer := call(usr.NewProc("MessageBoxW"), hwnd, ptr("退出 AutoMesh？是：退出；否：隐藏到托盘；取消：保留窗口。不会关闭 ANSYS。"), ptr("AutoMesh"), 0x23)
		if answer == 6 {
			lifecycleEvent("exit_requested", nil)
			call(usr.NewProc("DestroyWindow"), hwnd)
		} else if answer == 7 {
			hideToTray(hwnd)
		}
		return true
	case wmWorkerError:
		select {
		case message := <-workerErrors:
			ui.initRunning = false
			ui.connected = false
			ui.errorText = message
			ui.busySet(false)
			ui.message("后台操作异常；保留远端任务记录，请先检查任务状态。详情见 lifecycle.jsonl")
			restoreMainWindow(hwnd)
		default:
		}
		return true
	}
	if msg == taskbarCreated && taskbarCreated != 0 {
		trayPresent = false
		if !setTray(hwnd, true) {
			restoreMainWindow(hwnd)
		}
		return true
	}
	return false
}
func backgroundGuard() {
	if problem := recover(); problem != nil {
		message := fmt.Sprintf("%v\n%s", problem, debug.Stack())
		lifecycleEvent("worker_panic", message)
		select {
		case workerErrors <- message:
		default:
		}
		call(post, ui.window, wmWorkerError, 0, 0)
	}
}
func installCrashLog() func() {
	f, e := os.OpenFile(ui.ws.path("crash.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		lifecycleEvent("crash_log_error", e.Error())
		return func() {}
	}
	if e = debug.SetCrashOutput(f, debug.CrashOptions{}); e != nil {
		lifecycleEvent("crash_log_error", e.Error())
	}
	f.Close() // SetCrashOutput retains a duplicate handle.
	lifecycleEvent("startup", map[string]any{"version": "R8.1.2", "executable": os.Args[0]})
	return func() { lifecycleEvent("exit", nil); _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) }
}
