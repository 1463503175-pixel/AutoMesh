//go:build windows

package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

func TestRestoreHiddenNativeWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hwnd := call(create, 0, ptr("STATIC"), ptr("AutoMesh lifecycle test"), WS_OVERLAPPEDWINDOW, 0, 0, 400, 200, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal("CreateWindow failed")
	}
	defer call(usr.NewProc("DestroyWindow"), hwnd)
	call(show, hwnd, 6)
	restoreMainWindow(hwnd)
	if call(usr.NewProc("IsWindowVisible"), hwnd) == 0 || call(usr.NewProc("IsIconic"), hwnd) != 0 {
		t.Fatal("window did not restore")
	}
	call(show, hwnd, 0)
	restoreMainWindow(hwnd)
	if call(usr.NewProc("IsWindowVisible"), hwnd) == 0 {
		t.Fatal("hidden window did not restore")
	}
}

func TestTrayCallbackRestoresWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hwnd := call(create, 0, ptr("STATIC"), ptr("AutoMesh tray test"), WS_OVERLAPPEDWINDOW, 0, 0, 400, 200, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal("CreateWindow failed")
	}
	defer call(usr.NewProc("DestroyWindow"), hwnd)
	if unsafe.Sizeof(notifyIconData{}) != 976 {
		t.Fatal("wrong x64 tray ABI")
	}
	trayIcon = call(loadicon, 0, 32512)
	if !setTray(hwnd, true) {
		t.Skip("shell notification area unavailable")
	}
	defer setTray(hwnd, false)
	call(show, hwnd, 0)
	if !lifecycleMessage(hwnd, wmTrayEvent, 1, 0x202) || call(usr.NewProc("IsWindowVisible"), hwnd) == 0 {
		t.Fatal("tray click did not restore window")
	}
}
func TestWorkerPanicPreservesProcessAndLogs(t *testing.T) {
	previous := ui.ws
	ui.ws = Workspace{Dir: t.TempDir()}
	defer func() { ui.ws = previous }()
	func() { defer backgroundGuard(); panic("simulated worker failure") }()
	select {
	case message := <-workerErrors:
		if !strings.Contains(message, "simulated worker failure") {
			t.Fatal(message)
		}
	default:
		t.Fatal("missing error notification")
	}
	b, err := os.ReadFile(ui.ws.path("lifecycle.jsonl"))
	if err != nil || !strings.Contains(string(b), "worker_panic") {
		t.Fatalf("missing panic log: %v", err)
	}
}
