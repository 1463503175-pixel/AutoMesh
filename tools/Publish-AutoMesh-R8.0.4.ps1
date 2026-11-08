# AutoMesh R8.0.4 one-run publisher for the existing public GitHub repository.
# Requires Git for Windows only; GitHub CLI (gh) is not required.
# Safe behavior: clones the existing main branch, copies the release, commits and pushes normally.
# It never force-pushes, deletes remote history, or touches ANSYS/Workbench/Meshing.

[CmdletBinding()]
param(
    [string]$PackagePath = '',
    [string]$Repository = '1463503175-pixel/AutoMesh',
    [string]$Branch = 'main'
)

$ErrorActionPreference = 'Stop'

function Invoke-Git {
    param([string[]]$GitArgs, [string]$FailureMessage)
    & git @GitArgs
    if ($LASTEXITCODE -ne 0) { throw $FailureMessage }
}

function Copy-DirectoryContents {
    param([string]$Source, [string]$Destination)
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    Get-ChildItem -LiteralPath $Source -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $Destination -Recurse -Force
    }
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    throw '未检测到 Git。请先安装 Git for Windows。'
}

if ([string]::IsNullOrWhiteSpace($PackagePath)) {
    Add-Type -AssemblyName System.Windows.Forms
    $dialog = New-Object System.Windows.Forms.OpenFileDialog
    $dialog.Title = '选择 AutoMesh R8.0.4 完整 ZIP 发布包'
    $dialog.Filter = 'AutoMesh R8.0.4 ZIP (*.zip)|*.zip'
    $dialog.Multiselect = $false
    if ($dialog.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) {
        throw '未选择发布包，已取消。'
    }
    $PackagePath = $dialog.FileName
}

$PackagePath = (Resolve-Path -LiteralPath $PackagePath).Path
if ([IO.Path]::GetExtension($PackagePath) -ne '.zip') {
    throw '请选择 AutoMesh_R8_0_4_Continuous_Refinement.zip。'
}

$repoUrl = "https://github.com/$Repository.git"
$owner = ($Repository -split '/')[0]
$publishId = [guid]::NewGuid().ToString('N').Substring(0, 8)
$workRoot = Join-Path $env:TEMP "AutoMesh_R8.0.4_Publish_$publishId"
$extractRoot = Join-Path $workRoot 'package'
$repoRoot = Join-Path $workRoot 'repository'
New-Item -ItemType Directory -Path $extractRoot -Force | Out-Null

Write-Host '[1/7] 检查并解压 R8.0.4 发布包...' -ForegroundColor Cyan
Expand-Archive -LiteralPath $PackagePath -DestinationPath $extractRoot -Force
$sourceDir = Get-ChildItem -LiteralPath $extractRoot -Directory -Recurse -Force |
    Where-Object {
        $_.Name -eq '完整源码' -and (Test-Path -LiteralPath (Join-Path $_.Parent.FullName 'AutoMesh.exe'))
    } | Select-Object -First 1
if (-not $sourceDir) {
    throw 'ZIP 结构不符合预期：未找到“完整源码”目录及同级 AutoMesh.exe。'
}
$packageRoot = $sourceDir.Parent.FullName
$exePath = Join-Path $packageRoot 'AutoMesh.exe'
$releaseReadme = Join-Path $packageRoot 'README_R8.0.4.md'
$validationPath = Join-Path $packageRoot '验证结果_R8.0.4.txt'
$releaseNote = Join-Path $sourceDir.FullName 'R8.0.4_连续优化与历史Sizing接管.md'
if (-not (Test-Path -LiteralPath $exePath)) { throw '发布包缺少 AutoMesh.exe。' }
if (-not (Test-Path -LiteralPath $releaseNote)) { throw '发布包缺少 R8.0.4 修复说明。' }

Write-Host '[2/7] 克隆现有 GitHub 仓库（不会覆盖远端历史）...' -ForegroundColor Cyan
Invoke-Git -GitArgs @('clone', '--branch', $Branch, '--single-branch', $repoUrl, $repoRoot) -FailureMessage '克隆仓库失败。请检查网络及 GitHub 访问权限。'

Write-Host '[3/7] 更新程序、完整源码和版本资料...' -ForegroundColor Cyan
$appDir = Join-Path $repoRoot 'app'
$sourceTarget = Join-Path $repoRoot 'source'
$docsReleaseDir = Join-Path $repoRoot 'docs/releases'
$releaseDir = Join-Path $repoRoot 'releases'
New-Item -ItemType Directory -Path $appDir, $sourceTarget, $docsReleaseDir, $releaseDir -Force | Out-Null
Copy-Item -LiteralPath $exePath -Destination (Join-Path $appDir 'AutoMesh.exe') -Force
Copy-DirectoryContents -Source $sourceDir.FullName -Destination $sourceTarget
Copy-Item -LiteralPath $releaseNote -Destination (Join-Path $docsReleaseDir 'R8.0.4_连续优化与历史Sizing接管.md') -Force
if (Test-Path -LiteralPath $releaseReadme) {
    Copy-Item -LiteralPath $releaseReadme -Destination (Join-Path $docsReleaseDir 'README_R8.0.4.md') -Force
}
if (Test-Path -LiteralPath $validationPath) {
    Copy-Item -LiteralPath $validationPath -Destination (Join-Path $docsReleaseDir '验证结果_R8.0.4.txt') -Force
}
$archiveTarget = Join-Path $releaseDir 'AutoMesh_R8_0_4_Continuous_Refinement.zip'
Copy-Item -LiteralPath $PackagePath -Destination $archiveTarget -Force

Write-Host '[4/7] 更新仓库首页和 SHA-256 清单...' -ForegroundColor Cyan
$readmePath = Join-Path $repoRoot 'README.md'
$readme = Get-Content -LiteralPath $readmePath -Raw -Encoding UTF8
if ($readme -notmatch '最新版本：R8\.0\.4') {
    $latestBlock = @'

## 最新版本：R8.0.4

- Windows x64 程序：[app/AutoMesh.exe](app/AutoMesh.exe)
- 完整发布包：[releases/AutoMesh_R8_0_4_Continuous_Refinement.zip](releases/AutoMesh_R8_0_4_Continuous_Refinement.zip)
- 连续优化与历史 Sizing 接管说明：[docs/releases/R8.0.4_连续优化与历史Sizing接管.md](docs/releases/R8.0.4_连续优化与历史Sizing接管.md)
- 完整源码位于 source/。

首次使用请先阅读版本说明。离线测试不等同于 ANSYS 2024 R2 实机端到端验收；发布 Sizing 前仍须在当前 Meshing 会话核对历史控件与影响域。
'@
    $newlineToken = [Environment]::NewLine
    $newline = $readme.IndexOf($newlineToken)
    if ($newline -ge 0) {
        $readme = $readme.Substring(0, $newline + $newlineToken.Length) + $latestBlock + $newlineToken + $readme.Substring($newline + $newlineToken.Length)
    } else {
        $readme = $readme + $newlineToken + $latestBlock + $newlineToken
    }
    [IO.File]::WriteAllText($readmePath, $readme, [Text.UTF8Encoding]::new($false))
}

$manifestPath = Join-Path $repoRoot 'SHA256SUMS.txt'
$manifestLines = Get-ChildItem -LiteralPath $repoRoot -File -Recurse -Force |
    Where-Object { $_.FullName -notmatch '[\\/]\.git[\\/]' -and $_.Name -ne 'SHA256SUMS.txt' } |
    ForEach-Object {
        $relative = $_.FullName.Substring($repoRoot.TrimEnd('\').Length + 1).Replace('\', '/')
        $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $relative"
    } | Sort-Object
[IO.File]::WriteAllLines($manifestPath, [string[]]$manifestLines, [Text.UTF8Encoding]::new($false))

Write-Host '[5/7] 提交 R8.0.4（不使用 force push）...' -ForegroundColor Cyan
Invoke-Git -GitArgs @('-C', $repoRoot, 'add', '-A') -FailureMessage 'git add 失败。'
$changes = & git -C $repoRoot status --porcelain
if (-not $changes) { throw '没有检测到待提交文件；为避免空提交，已停止。' }
$name = (& git -C $repoRoot config user.name 2>$null)
if (-not $name) { Invoke-Git -GitArgs @('-C', $repoRoot, 'config', 'user.name', $owner) -FailureMessage '设置 Git 用户名失败。' }
$email = (& git -C $repoRoot config user.email 2>$null)
if (-not $email) { Invoke-Git -GitArgs @('-C', $repoRoot, 'config', 'user.email', "$owner+291213687@users.noreply.github.com") -FailureMessage '设置 Git 邮箱失败。' }
Invoke-Git -GitArgs @('-C', $repoRoot, 'commit', '-m', 'Release AutoMesh R8.0.4 continuous refinement') -FailureMessage 'git commit 失败；临时目录中的提交文件仍保留。'

Write-Host '[6/7] 推送到 GitHub。首次推送时 Git Credential Manager 可能会打开浏览器授权...' -ForegroundColor Cyan
Invoke-Git -GitArgs @('-C', $repoRoot, 'push', 'origin', $Branch) -FailureMessage 'Git push 失败。请完成 Git Credential Manager 登录后重新运行脚本。没有执行 force push。'

Write-Host '[7/7] 从远端核验提交...' -ForegroundColor Cyan
$localSha = (& git -C $repoRoot rev-parse HEAD).Trim()
$remoteLine = & git -C $repoRoot ls-remote origin "refs/heads/$Branch"
if ($LASTEXITCODE -ne 0 -or -not $remoteLine) { throw '推送后无法读取远端分支，需手动核验。' }
$remoteSha = (($remoteLine -split '\s+')[0]).Trim()
if ($localSha -ne $remoteSha) { throw "远端 SHA 与本地提交不一致。local=$localSha remote=$remoteSha" }

Write-Host ''
Write-Host 'R8.0.4 已推送并通过远端 SHA 核验。' -ForegroundColor Green
Write-Host "仓库：https://github.com/$Repository"
Write-Host "分支：$Branch"
Write-Host "提交：$localSha"
Write-Host "本地临时副本：$repoRoot"
Write-Host '本脚本仅更新 GitHub 文件；不会启动、重启或操作 ANSYS、Workbench、Meshing 或 Fluent。'
