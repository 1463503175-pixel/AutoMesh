# Read-only. Does not restart/terminate ANSYS, modify projects, or clear jobs.
$ErrorActionPreference = 'Continue'
$target = Join-Path $PSScriptRoot 'Meshing_Crash_Events.txt'
$events = Get-WinEvent -FilterHashtable @{LogName='Application'; Id=1000,1001; StartTime=(Get-Date).AddDays(-3)} -ErrorAction SilentlyContinue |
  Where-Object { $_.Message -match '(?i)ansys|meshing|ansyswbu|runwb2' } |
  Select-Object -First 20 TimeCreated, Id, ProviderName, Message
if ($events) {
  $events | Format-List | Out-File -FilePath $target -Encoding utf8
  Write-Host "Crash log saved: $target"
} else { Write-Host 'No matching Windows Application crash events were available in the last 3 days.' }
