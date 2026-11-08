# Read-only Windows UIAutomation inspection for R7.8.
# Never clicks, sends keys, edits ANSYS, captures screenshots or alters tasks.
$ErrorActionPreference = 'Stop'
try {
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    $proc = Get-Process -Name 'AutoMesh' -ErrorAction Stop |
        Where-Object { $_.MainWindowHandle -ne [IntPtr]::Zero -and $_.MainWindowTitle -like '*AutoMesh*' } |
        Select-Object -First 1
    if (-not $proc) { throw 'AutoMesh 主窗口未运行。请先启动 R7.9。' }
    $root = [System.Windows.Automation.AutomationElement]::FromHandle($proc.MainWindowHandle)
    $nodes = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.Condition]::TrueCondition)
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('AutoMesh R7.9 - READ-ONLY UIAutomation Audit')
    $lines.Add('Time: ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'))
    $lines.Add('Process ID: ' + $proc.Id)
    $lines.Add('Window: ' + $proc.MainWindowTitle)
    $lines.Add('Window rect: ' + $root.Current.BoundingRectangle.ToString())
    $lines.Add('Note: open each page, rerun this script; inspect 100/125/150 percent DPI separately.')
    $lines.Add('No screenshot clicks or input automation is used.')
    $lines.Add('')
    for ($i = 0; $i -lt $nodes.Count; $i++) {
        $n=$nodes.Item($i)
        try {
            $info=$n.Current
            if ($info.ControlType -ne [System.Windows.Automation.ControlType]::Button -and
                $info.ControlType -ne [System.Windows.Automation.ControlType]::Edit -and
                $info.ControlType -ne [System.Windows.Automation.ControlType]::ComboBox -and
                $info.ControlType -ne [System.Windows.Automation.ControlType]::List) { continue }
            $entry = ('{0} | Enabled={1} | Offscreen={2} | Rect={3} | Name={4}' -f
                $info.ControlType.ProgrammaticName, $info.IsEnabled, $info.IsOffscreen,
                $info.BoundingRectangle.ToString(), $info.Name)
            $lines.Add($entry)
        } catch { $lines.Add('Inaccessible control: ' + $_.Exception.Message) }
    }
    $out=Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'AutoMesh_UI_Inspection.txt'
    $lines | Set-Content -LiteralPath $out -Encoding UTF8
    Write-Host ('Read-only UI inspection saved: ' + $out)
} catch {
    Write-Error ('UIAutomation inspection unavailable: ' + $_.Exception.Message)
    exit 1
}
