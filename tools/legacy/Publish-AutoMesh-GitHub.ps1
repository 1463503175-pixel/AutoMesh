# AutoMesh R8.0 - publish to the existing PUBLIC GitHub repository.
# Safe behavior: verifies account, visibility, permissions and empty remote; never force-pushes.
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

$repo = '1463503175-pixel/AutoMesh'
$owner = '1463503175-pixel'
$remoteUrl = "https://github.com/$repo.git"

function Assert-LastExitCode([string]$message) {
    if ($LASTEXITCODE -ne 0) { throw $message }
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    throw '未检测到 Git。请先安装 Git for Windows，然后重新打开 PowerShell。'
}
if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw '未检测到 GitHub CLI (gh)。请安装 GitHub CLI：https://cli.github.com/ ，然后重新打开 PowerShell。'
}
if (-not (Test-Path -LiteralPath '.\AutoMesh.exe') -or -not (Test-Path -LiteralPath '.\完整源码')) {
    throw '当前目录不是完整 AutoMesh 发布包根目录。请先完整解压 ZIP，再在该目录运行脚本。'
}

Write-Host '[1/6] 检查 GitHub 登录状态...' -ForegroundColor Cyan
& gh auth status
Assert-LastExitCode 'GitHub CLI 尚未完成登录。请先执行：gh auth login'
$login = (& gh api user --jq '.login').Trim()
Assert-LastExitCode '无法读取当前 GitHub 登录账号。'
if ($login -ne $owner) {
    throw "当前登录账号为 $login，不是目标账号 $owner。请切换 gh 登录账号后重试。"
}

# Configure Git to reuse the already-verified gh authentication for HTTPS push.
& gh auth setup-git
Assert-LastExitCode '配置 GitHub CLI 凭据助手失败。'

Write-Host '[2/6] 核验目标仓库...' -ForegroundColor Cyan
$repoJson = & gh api "repos/$repo" --jq '{visibility: .visibility, default_branch: .default_branch, can_push: .permissions.push, is_admin: .permissions.admin}'
Assert-LastExitCode '无法读取目标仓库信息，停止上传。'
$repoInfo = $repoJson | ConvertFrom-Json
if ($repoInfo.visibility -ne 'public') {
    throw "目标仓库可见性不是 Public（当前：$($repoInfo.visibility)），不会擅自更改仓库设置。"
}
if (-not $repoInfo.can_push -or -not $repoInfo.is_admin) {
    throw '当前账号缺少目标仓库 Push/Admin 权限。'
}
if ($repoInfo.default_branch -ne 'main') {
    throw "目标仓库默认分支为 $($repoInfo.default_branch)，与本次预期 main 不一致，停止以避免推错分支。"
}

Write-Host '[3/6] 检查远端是否为空...' -ForegroundColor Cyan
$branchCountText = & gh api "repos/$repo/branches?per_page=100" --jq 'length'
Assert-LastExitCode '无法核验远端分支，停止上传。'
$branchCount = [int]$branchCountText.Trim()
if ($branchCount -ne 0) {
    throw "远端已存在 $branchCount 个分支。本脚本不会覆盖或合并未知内容；请先检查仓库。"
}

Write-Host '[4/6] 初始化本地 Git 并提交完整文件...' -ForegroundColor Cyan
if (-not (Test-Path -LiteralPath '.git')) {
    & git init -b main
    Assert-LastExitCode 'git init 失败。'
} else {
    $currentBranch = (& git branch --show-current).Trim()
    if ($currentBranch -ne 'main') { throw "本地分支为 $currentBranch，不是 main；停止以避免推错分支。" }
}

$existingRemote = & git remote get-url origin 2>$null
if ($LASTEXITCODE -eq 0 -and $existingRemote) {
    if ($existingRemote.Trim().TrimEnd('/') -ne $remoteUrl.TrimEnd('/')) {
        throw "本地 origin 指向其他仓库：$existingRemote。不会自动覆盖。"
    }
} else {
    & git remote add origin $remoteUrl
    Assert-LastExitCode '添加 GitHub origin 失败。'
}

$name = (& git config user.name 2>$null)
if (-not $name) { & git config --local user.name $owner; Assert-LastExitCode '设置 Git 用户名失败。' }
$email = (& git config user.email 2>$null)
if (-not $email) {
    $uid = (& gh api user --jq '.id').Trim()
    Assert-LastExitCode '读取 GitHub 用户 ID 失败。'
    & git config --local user.email ("$uid+$owner@users.noreply.github.com")
    Assert-LastExitCode '设置 Git 邮箱失败。'
}

& git add -A
Assert-LastExitCode 'git add 失败。'
& git diff --cached --quiet
$hasStagedChanges = ($LASTEXITCODE -ne 0)
& git rev-parse --verify HEAD 2>$null | Out-Null
$hasCommit = ($LASTEXITCODE -eq 0)
if ($hasStagedChanges -or -not $hasCommit) {
    & git commit -m 'Publish AutoMesh R8.0 complete source and Windows release'
    Assert-LastExitCode 'git commit 失败；本地文件仍保留。'
}

Write-Host '[5/6] 推送到现有 Public 仓库...' -ForegroundColor Cyan
& git push -u origin main
Assert-LastExitCode 'Git push 失败。请检查网络或 Git Credential Manager 登录状态；本地提交不会丢失。'

Write-Host '[6/6] 从 GitHub 核验上传结果...' -ForegroundColor Cyan
$commit = (& gh api "repos/$repo/branches/main" --jq '.commit.sha').Trim()
Assert-LastExitCode '推送后无法读取远端 main 分支提交。'
$treeCountText = & gh api "repos/$repo/git/trees/$commit?recursive=1" --jq '.tree | length'
Assert-LastExitCode '推送后无法读取远端文件树。'
$treeCount = [int]$treeCountText.Trim()
if ($treeCount -lt 100) { throw "远端文件树仅 $treeCount 项，少于预期；请检查 GitHub 仓库。" }

Write-Host ''
Write-Host '上传完成并已从 GitHub 核验。' -ForegroundColor Green
Write-Host "仓库：https://github.com/$repo"
Write-Host "提交：$commit"
Write-Host "远端文件/目录项：$treeCount"
Write-Host '注意：本次只上传文件，不会运行 AutoMesh、ANSYS 或修改任何工程。'
