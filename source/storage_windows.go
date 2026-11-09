//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

type browseInfo struct {
	owner, root uintptr
	display     *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	data        uintptr
	image       int32
}

func chooseDataParent(hwnd uintptr) (string, error) {
	shell := syscall.NewLazyDLL("shell32.dll")
	browse := shell.NewProc("SHBrowseForFolderW")
	pathProc := shell.NewProc("SHGetPathFromIDListW")
	ole := syscall.NewLazyDLL("ole32.dll")
	free := ole.NewProc("CoTaskMemFree")
	display := make([]uint16, 260)
	info := browseInfo{owner: hwnd, display: &display[0], title: syscall.StringToUTF16Ptr("选择新数据目录的父文件夹（仅英文路径，建议 D/E 盘）"), flags: 0x0040 | 0x0001}
	pidl, _, _ := browse.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return "", nil
	} // Cancel, not an error
	defer free.Call(pidl)
	dest := make([]uint16, 32768)
	ok, _, _ := pathProc.Call(pidl, uintptr(unsafe.Pointer(&dest[0])))
	if ok == 0 {
		return "", fmt.Errorf("Windows cannot resolve selected folder")
	}
	return syscall.UTF16ToString(dest), nil
}

// Migration is initiated from a real UI button. Never manipulate a live
// ANSYS task or replace a user's existing destination directory.
func (a *state) changeDataDirectory() {
	if a.busy {
		alert("当前有 AutoMesh 任务在执行，禁止迁移数据")
		return
	}
	if _, err := os.Lstat(a.ws.taskLeasePath()); err == nil {
		alert("检测到未结束的任务占用记录，请先确认当前 Meshing 已停止，再处理任务记录；数据没有迁移。")
		return
	} else if !os.IsNotExist(err) {
		alert(err.Error())
		return
	}
	base, e := chooseDataParent(a.window)
	if e != nil {
		alert(e.Error())
		return
	}
	if base == "" {
		return
	}
	target := filepath.Join(base, "AutoMesh2024R2", "Data")
	if err := validateDataDirectory(target); err != nil {
		alert(err.Error())
		return
	}
	if strings.EqualFold(filepath.Clean(target), filepath.Clean(a.ws.Dir)) {
		alert("已经是当前数据目录")
		return
	}
	if !confirm("仅迁移 AutoMesh 任务、报告、日志和设置，不涉及 ANSYS 工程或网格。\n\n旧目录：" + a.ws.Dir + "\n新目录：" + target + "\n\n先复制并逐文件核验，成功后切换。旧数据保留，不自动删除。目标目录必须尚不存在。继续？") {
		return
	}
	a.busySet(true)
	a.postProgress("I|迁移数据|复制、逐文件校验后切换；旧数据保留")
	old := a.ws.Dir
	go func() {
		n, err := copyWorkspaceVerified(old, target)
		if err == nil {
			err = (Workspace{Dir: target}).InstallAssets()
		}
		if err == nil {
			_, err = saveConfiguredDataDirectory(target)
		}
		a.mu.Lock()
		a.storageError = err
		if err == nil {
			a.storageDoneDir = target
			a.storageDoneFiles = n
		} else {
			a.storageDoneDir = ""
		}
		a.mu.Unlock()
		call(post, a.window, WM_APPSTORAGE, 0, 0)
	}()
}
func (a *state) finishStorage() {
	a.mu.Lock()
	e := a.storageError
	target := a.storageDoneDir
	n := a.storageDoneFiles
	a.mu.Unlock()
	a.busySet(false)
	if e != nil {
		a.message("存储迁移失败；原路径未修改：" + shorten(e.Error(), 120))
		alert("迁移失败：" + e.Error() + "\n原数据完整保留。若目标产生残留，请人工核对后处理。")
		return
	}
	a.ws = Workspace{Dir: target}
	// A new path means new command scripts and task records. Reconnecting is
	// required; never reuse an old Session/Snapshot association silently.
	a.connected = false
	a.scannedSystem = ""
	a.preparedSystem = ""
	a.snapshot = nil
	a.namedRows = nil
	a.selectedNamed = nil
	a.pausedNamedSizes = nil
	a.qualityResult = nil
	a.qualitySummary = nil
	a.qualityStale = true
	a.message(fmt.Sprintf("已迁移校验 %d 个文件，工作路径：%s；请重新连接并扫描", n, target))
	alert(fmt.Sprintf("数据迁移成功（校验 %d 个文件）。\n新路径：%s\n旧路径保留。\n请重新连接现有 Workbench 并扫描。", n, target))
	a.switchPage(5)
}
