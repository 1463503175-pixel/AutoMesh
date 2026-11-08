param([string]$DataDir = '')
$ErrorActionPreference = 'Continue'
if ([string]::IsNullOrWhiteSpace($DataDir)) {
    $DataDir = Join-Path $env:ProgramData 'AutoMesh2024R2\Data'
}
$lines = New-Object System.Collections.Generic.List[string]
$lines.Add('AutoMesh READ-ONLY task diagnosis')
$lines.Add('Directory: ' + $DataDir)
$lines.Add('Checked: ' + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'))
foreach ($name in @('active_task.json','last_result.json','startup_diagnostic.json')) {
    $path = Join-Path $DataDir $name
    $lines.Add('=== ' + $name + ' ===')
    if (Test-Path -LiteralPath $path) {
        try { $lines.Add((Get-Content -LiteralPath $path -Raw -Encoding UTF8)) }
        catch { $lines.Add('Read failure: ' + $_.Exception.Message) }
    } else { $lines.Add('Not found') }
}
$items = @(Get-ChildItem -LiteralPath $DataDir -File -ErrorAction SilentlyContinue |
   Where-Object { $_.Name -match '^(job_[0-9a-f]{32}\.json|job_progress_[0-9a-f]{32}\.json|job_receipt_write_error_.*\.txt|warning_publish_partial\.json|warning_refinement_managed\.json)$' } |
   Sort-Object LastWriteTime -Descending | Select-Object -First 12)
foreach ($f in $items) {
    $lines.Add('=== ' + $f.Name + ' ===')
    try { $content = (Get-Content -LiteralPath $f.FullName -Raw -Encoding UTF8); $lines.Add($content.Substring(0,[Math]::Min(12000,$content.Length))) }
    catch { $lines.Add('Read failure: '+$_.Exception.Message) }
}
$out = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'AutoMesh_Task_Diagnostic.txt'
$lines | Set-Content -LiteralPath $out -Encoding UTF8
Write-Host ('Saved read-only diagnostic: ' + $out)
Write-Host 'No task was cancelled and no files in the AutoMesh data directory were changed.'
