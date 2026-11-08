# AutoMesh: opt-in, existing-process-only Workbench command-window bootstrap.
# No screenshot recognition, screen coordinates, new Workbench process, or project changes.
param([ValidateSet('start','probe')][string]$Mode='start')
$ErrorActionPreference = 'Stop'
try { [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false) } catch {}
function Result([string]$status,[string]$detail,[int]$procId=0) {
    @{status=$status; detail=$detail; process_id=$procId} | ConvertTo-Json -Compress -Depth 4
}
try {
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class AutoMeshForeground {
 [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);
}
'@
    $desktop = [System.Windows.Automation.AutomationElement]::RootElement
    $ts = [System.Windows.Automation.TreeScope]
    $ae = [System.Windows.Automation.AutomationElement]
    $match = @()
    foreach($w in $desktop.FindAll($ts::Children,[System.Windows.Automation.Condition]::TrueCondition)) {
        try {
            $wid = [int]$w.Current.ProcessId
            if ($wid -le 0) { continue }
            $proc = [System.Diagnostics.Process]::GetProcessById($wid)
            $exe = $proc.ProcessName.ToLowerInvariant()
            if ($exe -notin @('runwb2','ansyswbu','ansysfw','ansysfww')) { continue }
            $name = [string]$w.Current.Name
            if ($name -match '(?i)Workbench|工作台') { $match += $w }
        } catch { continue }
    }
    $processes = @($match | ForEach-Object { [int]$_.Current.ProcessId } | Select-Object -Unique)
    if ($processes.Count -ne 1) {
        Result 'unavailable' ("必须唯一识别正在运行的 Workbench 主窗口；匹配进程数=" + $processes.Count) 0
        exit 0
    }
    $targetPid = [int]$processes[0]
    if ($Mode -eq 'probe') {
        Result 'identified' '已识别当前 Workbench 进程；尚未执行命令' $targetPid
        exit 0
    }
    $win = @($match | Where-Object { $_.Current.ProcessId -eq $targetPid })[0]
    # Exact menu-node selection through accessibility metadata, not coordinates.
    function NamedElements([string]$namePattern) {
        $out = @()
        $allWindows = $desktop.FindAll($ts::Children,[System.Windows.Automation.Condition]::TrueCondition)
        foreach($top in $allWindows) {
            try { if ([int]$top.Current.ProcessId -ne $targetPid) { continue } } catch { continue }
            $elems = $top.FindAll($ts::Descendants,[System.Windows.Automation.Condition]::TrueCondition)
            foreach($el in $elems) {
                try { if ([string]$el.Current.Name -match $namePattern) { $out += $el } } catch {}
            }
            if ([string]$top.Current.Name -match $namePattern) { $out += $top }
        }
        return $out
    }
    function CommandPanes {
        # Only a live Command Window (including floating docked views) is eligible.
        $all = NamedElements '(?i)^(Command Window|命令窗口|命令窗格)$'
        return @($all | Where-Object {
            $_.Current.ControlType -eq [System.Windows.Automation.ControlType]::Pane -or
            $_.Current.ControlType -eq [System.Windows.Automation.ControlType]::Window -or
            $_.Current.ControlType -eq [System.Windows.Automation.ControlType]::Group
        })
    }
    function OpenUiMenu([string]$pattern) {
        $hits = @(NamedElements $pattern | Where-Object { $_.Current.ControlType -eq [System.Windows.Automation.ControlType]::MenuItem })
        if ($hits.Count -ne 1) { return $false }
        $elem=$hits[0]
        try {
            $p=$elem.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern)
            $p.Expand(); Start-Sleep -Milliseconds 300; return $true
        } catch {}
        try {
            $p=$elem.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
            $p.Invoke(); Start-Sleep -Milliseconds 300; return $true
        } catch {}
        return $false
    }
    $panes = @(CommandPanes)
    if ($panes.Count -eq 0) {
        # Documented path: File > Scripting > Open Command Window.
        # If this menu is not exposed to UIA we refuse; no guessed hotkeys.
        $fileOpen = OpenUiMenu '(?i)^(File|文件)$'
        if ($fileOpen) {
            $scriptOpen = OpenUiMenu '(?i)^(Scripting|脚本|脚本编写)$'
            if ($scriptOpen) {
                [void](OpenUiMenu '(?i)^(Open Command Window|打开命令窗口)$')
                Start-Sleep -Milliseconds 600
            }
        }
        $panes = @(CommandPanes)
    }
    if ($panes.Count -ne 1) {
        Result 'unavailable' ("未能唯一定位 Workbench 命令窗口（数量=" + $panes.Count + "）。没有向任何输入框发送指令。") $targetPid
        exit 0
    }
    $cmdWindow = $panes[0]
    # Only an editable ValuePattern inside the *verified Command Window* is safe.
    $eligible=@()
    foreach($ctrl in $cmdWindow.FindAll($ts::Descendants,[System.Windows.Automation.Condition]::TrueCondition)) {
        try {
            if ($ctrl.Current.ControlType -ne [System.Windows.Automation.ControlType]::Edit -or -not $ctrl.Current.IsEnabled) { continue }
            $valuePattern=[System.Windows.Automation.ValuePattern]$ctrl.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
            if ($valuePattern.Current.IsReadOnly) { continue }
            $eligible+=@(@{control=$ctrl; pattern=$valuePattern})
        } catch { continue }
    }
    if ($eligible.Count -ne 1) {
        Result 'unavailable' ("命令窗口可编辑输入框不唯一或不支持安全写入（数量=" + $eligible.Count + "）。") $targetPid
        exit 0
    }
    $edit = $eligible[0].control
    $valuePattern = $eligible[0].pattern
    if (-not [string]::IsNullOrWhiteSpace([string]$valuePattern.Current.Value)) {
        Result 'unavailable' '命令输入框已有未执行文本，禁止覆盖用户输入。' $targetPid
        exit 0
    }
    # The caller may be foreground. Request focus on the already-running
    # Workbench (never clicks or locates pixels) before typing.
    try { [void][AutoMeshForeground]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) } catch {}
    Start-Sleep -Milliseconds 100
    $edit.SetFocus()
    Start-Sleep -Milliseconds 150
    $focusPid = [uint32]0
    $current = [AutoMeshForeground]::GetForegroundWindow()
    [void][AutoMeshForeground]::GetWindowThreadProcessId($current,[ref]$focusPid)
    if ([int]$focusPid -ne $targetPid -or
        -not [System.Windows.Automation.AutomationElement]::Compare([System.Windows.Automation.AutomationElement]::FocusedElement,$edit)) {
        Result 'unavailable' '无法确认键盘焦点属于唯一的 Workbench 命令输入控件，未提交指令。' $targetPid
        exit 0
    }
    $valuePattern.SetValue('StartServer()')
    Start-Sleep -Milliseconds 80
    # Validate focus again immediately before the Enter keystroke.
    $focusPid = [uint32]0
    [void][AutoMeshForeground]::GetWindowThreadProcessId([AutoMeshForeground]::GetForegroundWindow(),[ref]$focusPid)
    if ([int]$focusPid -ne $targetPid -or
        -not [System.Windows.Automation.AutomationElement]::Compare([System.Windows.Automation.AutomationElement]::FocusedElement,$edit)) {
        Result 'unavailable' '输入后焦点发生变化；未发送回车，请检查 Workbench 命令框。' $targetPid
        exit 0
    }
    [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
    Result 'submitted' '已向唯一匹配的 Workbench 命令窗口提交 StartServer()；仍需核验服务端口。' $targetPid
} catch {
    Result 'unavailable' ('控件接口不可用：' + $_.Exception.Message) 0
}
