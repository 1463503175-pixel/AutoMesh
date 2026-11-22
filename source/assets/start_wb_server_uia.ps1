# AutoMesh: existing-process-only Workbench command-window bootstrap.
# No screenshot recognition, screen coordinates, new Workbench process, or project changes.
param([ValidateSet('start','probe')][string]$Mode='start')
$ErrorActionPreference = 'Stop'
try { [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false) } catch {}
function Result([string]$status,[string]$detail,[int]$procId=0,[int]$serverPort=0) {
    @{status=$status; detail=$detail; process_id=$procId; server_port=$serverPort} | ConvertTo-Json -Compress -Depth 4
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

    Add-Type -AssemblyName Accessibility
    Add-Type -ReferencedAssemblies Accessibility -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text.RegularExpressions;
using Accessibility;
public static class AutoMeshAccessibleMenu {
 [DllImport("oleacc.dll")] static extern int AccessibleObjectFromWindow(IntPtr hwnd,uint objectId,ref Guid iid,[MarshalAs(UnmanagedType.Interface)]out object result);
 [DllImport("oleacc.dll")] static extern int AccessibleChildren(IAccessible parent,int start,int count,[Out,MarshalAs(UnmanagedType.LPArray,SizeParamIndex=2)]object[] children,out int obtained);
 class Entry {public IAccessible Parent;public object Child;}
 static void Match(IAccessible a,object child,string pattern,List<Entry> hits,HashSet<string> seen){
  try {string name=a.get_accName(child);if(name==null||!Regex.IsMatch(name,pattern,RegexOptions.IgnoreCase))return;
   IntPtr identity=Marshal.GetIUnknownForObject(a);string key;
   try{key=identity.ToInt64()+":"+child;}finally{Marshal.Release(identity);}
   if(seen.Add(key))hits.Add(new Entry{Parent=a,Child=child});
  }catch{}
 }
 static void Walk(IAccessible a,int depth,string pattern,List<Entry> hits,HashSet<string> seen,ref int count){
  if(depth>16||++count>12000)return;Match(a,0,pattern,hits,seen);
  int n;try{n=a.accChildCount;}catch{return;}if(n<=0)return;n=Math.Min(n,2048);
  object[] children=new object[n];int got;if(AccessibleChildren(a,0,n,children,out got)<0)return;
  for(int i=0;i<got;i++){IAccessible nested=children[i] as IAccessible;
   if(nested!=null)Walk(nested,depth+1,pattern,hits,seen,ref count);else Match(a,children[i],pattern,hits,seen);}
 }
 public static int InvokeNamed(IntPtr hwnd,string pattern){
  Guid iid=new Guid("618736e0-3c3d-11cf-810c-00aa00389b71");object root;
  if(AccessibleObjectFromWindow(hwnd,0xfffffffc,ref iid,out root)<0)return 0;
  var hits=new List<Entry>();var seen=new HashSet<string>();int count=0;
  Walk((IAccessible)root,0,pattern,hits,seen,ref count);
  if(hits.Count==1)hits[0].Parent.accDoDefaultAction(hits[0].Child);
  return hits.Count;
 }
}
'@
    $panes = @(CommandPanes)
    if ($panes.Count -eq 0) {
        # Documented path: File > Scripting > Open Command Window.
        # UIA is preferred; MSAA below handles menus not exposed to UIA.
        $fileOpen = OpenUiMenu '(?i)^(File|文件)$'
        if ($fileOpen) {
            $scriptOpen = OpenUiMenu '(?i)^(Scripting|脚本|脚本编写)$'
            if ($scriptOpen) {
                [void](OpenUiMenu '(?i)^(Open Command Window|打开命令窗口)$')
                Start-Sleep -Milliseconds 600
            }
        }
        $panes = @(CommandPanes)
        if ($panes.Count -eq 0) {
            # WinForms/Infragistics menus may exist only in MSAA, not UIA.
            # Invoke exact accessible names in this verified process; never use positions.
            $handle=[IntPtr]$win.Current.NativeWindowHandle
            $commandName='^(Open Command Window|\u6253\u5f00\u547d\u4ee4\u7a97\u53e3)$'
            $opened=[AutoMeshAccessibleMenu]::InvokeNamed($handle,$commandName)
            if ($opened -eq 0) {
                $fileCount=[AutoMeshAccessibleMenu]::InvokeNamed($handle,'^(File|\u6587\u4ef6)$')
                if ($fileCount -eq 1) {
                    Start-Sleep -Milliseconds 200
                    $scriptCount=[AutoMeshAccessibleMenu]::InvokeNamed($handle,'^(Scripting|\u811a\u672c|\u811a\u672c\u7f16\u5199)$')
                    if ($scriptCount -eq 1) {
                        Start-Sleep -Milliseconds 200
                        $opened=[AutoMeshAccessibleMenu]::InvokeNamed($handle,$commandName)
                    }
                }
            }
            if ($opened -eq 1) { Start-Sleep -Milliseconds 350 }
            $panes = @(CommandPanes)
        }
    }
    if ($panes.Count -ne 1) {
        Result 'unavailable' ("未能唯一定位 Workbench 命令窗口（数量=" + $panes.Count + "）。没有向任何输入框发送指令。") $targetPid
        exit 0
    }

    Add-Type -TypeDefinition @'
using System;using System.Text;using System.Collections.Generic;using System.Runtime.InteropServices;
public static class AutoMeshNativeConsole {
 public delegate bool EnumProc(IntPtr h,IntPtr l);
 [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr h,EnumProc p,IntPtr l);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] static extern int GetClassName(IntPtr h,StringBuilder b,int n);
 [DllImport("user32.dll")] static extern int GetWindowLong(IntPtr h,int index);
 [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr h,out uint pid);
 [DllImport("kernel32.dll")] static extern uint GetCurrentThreadId();
 [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll")] static extern bool PostMessage(IntPtr h,uint m,IntPtr w,IntPtr l);
 [DllImport("user32.dll")] static extern bool AttachThreadInput(uint from,uint to,bool attach);
 [DllImport("user32.dll")] static extern IntPtr SetFocus(IntPtr h);
 [DllImport("user32.dll")] static extern bool GetGUIThreadInfo(uint thread,ref GUIInfo info);
 [DllImport("user32.dll",CharSet=CharSet.Unicode,EntryPoint="SendMessageTimeoutW")] static extern IntPtr TextMessage(IntPtr h,uint m,IntPtr w,StringBuilder b,uint flags,uint timeout,out IntPtr result);
 [DllImport("user32.dll",EntryPoint="SendMessageTimeoutW")] static extern IntPtr Message(IntPtr h,uint m,IntPtr w,IntPtr l,uint flags,uint timeout,out IntPtr result);
 [StructLayout(LayoutKind.Sequential)] struct GUIInfo {public int size,flags;public IntPtr active,focus,capture,menu,movesize,caret;public int left,top,right,bottom;}
 public static IntPtr[] Inputs(IntPtr parent,int expectedPid){var list=new List<IntPtr>();EnumChildWindows(parent,(h,l)=>{uint pid;GetWindowThreadProcessId(h,out pid);var name=new StringBuilder(256);GetClassName(h,name,256);if(pid==expectedPid&&name.ToString().IndexOf("RICHEDIT",StringComparison.OrdinalIgnoreCase)>=0&&(GetWindowLong(h,-16)&2048)==0)list.Add(h);return true;},IntPtr.Zero);return list.ToArray();}
 public static string Text(IntPtr h){IntPtr len;if(Message(h,14,IntPtr.Zero,IntPtr.Zero,2,1500,out len)==IntPtr.Zero)throw new Exception("Console text length unavailable");if(len.ToInt64()>1048576)throw new Exception("Console history too large for safe check");var text=new StringBuilder((int)len+1);IntPtr read;if(TextMessage(h,13,new IntPtr(text.Capacity),text,2,1500,out read)==IntPtr.Zero)throw new Exception("Console text unavailable");return text.ToString();}
 public static void FocusEnd(IntPtr h){uint pid;uint thread=GetWindowThreadProcessId(h,out pid);uint mine=GetCurrentThreadId();bool attached=mine!=thread&&AttachThreadInput(mine,thread,true);try{SetFocus(h);}finally{if(attached)AttachThreadInput(mine,thread,false);}if(!Focused(h))throw new Exception("Native console focus not confirmed");IntPtr result;if(Message(h,177,new IntPtr(-1),new IntPtr(-1),2,1500,out result)==IntPtr.Zero)throw new Exception("Console caret unavailable");}
 public static bool Focused(IntPtr h){uint pid,foregroundPid;uint thread=GetWindowThreadProcessId(h,out pid);GetWindowThreadProcessId(GetForegroundWindow(),out foregroundPid);var info=new GUIInfo();info.size=Marshal.SizeOf(info);return foregroundPid==pid&&GetGUIThreadInfo(thread,ref info)&&info.focus==h;}
 public static void TypeCommand(IntPtr h){if(!Focused(h))throw new Exception("Console not foreground/focused");foreach(char c in "StartServer()") {IntPtr result;if(Message(h,258,new IntPtr(c),new IntPtr(1),2,1500,out result)==IntPtr.Zero)throw new Exception("Console character delivery unconfirmed; Enter not sent");}}
 public static void Enter(IntPtr h){if(!Focused(h))throw new Exception("Console focus changed; Enter not sent");if(!PostMessage(h,256,new IntPtr(13),new IntPtr(1)))throw new Exception("Enter delivery unconfirmed");PostMessage(h,257,new IntPtr(13),new IntPtr(unchecked((int)0xC0000001)));}
}
'@
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
    if ($eligible.Count -eq 0 -and $cmdWindow.Current.NativeWindowHandle -ne 0) {
        $nativeInputs=@([AutoMeshNativeConsole]::Inputs([IntPtr]$cmdWindow.Current.NativeWindowHandle,$targetPid))
        if ($nativeInputs.Count -eq 1) {
            $consoleText=[AutoMeshNativeConsole]::Text($nativeInputs[0])
            if ($consoleText -notmatch '(?s)(^|\r?\n)>>> $') {
                Result 'unavailable' 'Native console has no empty primary prompt; user input was preserved.' $targetPid
                exit 0
            }
            $eligible+=@{native=$nativeInputs[0]; original=$consoleText}
        }
    }
    if ($eligible.Count -ne 1) {
        Result 'unavailable' ("命令窗口可编辑输入框不唯一或不支持安全写入（数量=" + $eligible.Count + "）。") $targetPid
        exit 0
    }
    if ($eligible[0].ContainsKey('native')) {
        $native=$eligible[0].native
        [void][AutoMeshForeground]::SetForegroundWindow([IntPtr]$cmdWindow.Current.NativeWindowHandle)
        [AutoMeshNativeConsole]::FocusEnd($native)
        if ([AutoMeshNativeConsole]::Text($native) -cne $eligible[0].original) { throw 'Console changed before submission; no input sent' }
        [AutoMeshNativeConsole]::TypeCommand($native)
        Start-Sleep -Milliseconds 100
        if (-not [AutoMeshNativeConsole]::Focused($native) -or
            [AutoMeshNativeConsole]::Text($native) -cne ($eligible[0].original+'StartServer()')) {
            Result 'unavailable' 'Console input/focus readback mismatch; Enter was not sent. Inspect pending input before retry.' $targetPid
            exit 0
        }
        [AutoMeshNativeConsole]::Enter($native)
        $serverPort=0
        for($attempt=0;$attempt -lt 20;$attempt++) {
            Start-Sleep -Milliseconds 100
            $response=[AutoMeshNativeConsole]::Text($native)
            if ($response.StartsWith($eligible[0].original,[StringComparison]::Ordinal)) {
                $tail=$response.Substring($eligible[0].original.Length)
                if ($tail -match '^StartServer\(\)\r?\n\s*(\d+)\s*\r?\n>>> $') {
                    $serverPort=[int]$matches[1]
                    if($serverPort -ge 1024 -and $serverPort -le 65535) {break}
                    $serverPort=0
                }
            }
        }
        Result 'submitted' 'StartServer() submitted through verified native RichEdit console; port verification still required.' $targetPid $serverPort
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
