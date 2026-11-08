# AutoMesh R8.0 — GitHub Public 上传说明

目标仓库已经由用户创建并设为 **Public**：

https://github.com/1463503175-pixel/AutoMesh

已核验目标账号具有 Admin / Push 权限，仓库当前为空。本说明与脚本专门针对这个**现有公开仓库**，不要求修改仓库可见性，也不会创建第二个仓库。

## 一次性准备

1. Windows 安装 [Git for Windows](https://git-scm.com/download/win)。
2. Windows 安装 [GitHub CLI](https://cli.github.com/)。
3. 打开 PowerShell，执行：

```powershell
gh auth login
```

选择 `GitHub.com`、`HTTPS`，按提示使用浏览器登录账号 `1463503175-pixel`。上传脚本会调用 `gh auth setup-git`，让 Git 推送复用已验证的登录凭据。

## 上传

1. 将本 ZIP 完整解压到英文路径，例如 `C:\AutoMesh_R8_0_GitHub_Public`。
2. 在解压后的根目录打开 PowerShell（该目录应能看到 `AutoMesh.exe`、`完整源码` 和本脚本）。
3. 执行：

```powershell
.\Publish-AutoMesh-GitHub.ps1
```

如果 PowerShell 阻止本次脚本，可仅对当前命令进程执行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\Publish-AutoMesh-GitHub.ps1
```

## 脚本安全检查

- 核验 GitHub 登录账号必须是 `1463503175-pixel`。
- 核验仓库必须是 Public、默认分支为 `main`，且当前账号有 Admin / Push 权限。
- 核验远端没有分支/提交；若已有内容，立即停止，不覆盖、不强推。
- 只推送当前解压目录中的 AutoMesh 发布内容到既有仓库。
- 推送后再从 GitHub 读取分支提交和文件树数量进行验收。
- 不修改仓库可见性，不创建新仓库，不运行 AutoMesh 或 ANSYS，不触碰本机工程。

## 包含内容

- Windows x64 `AutoMesh.exe`
- Go 源码、Python / ANSYS Worker、测试和历史版本说明
- R8.0 集成发布 ZIP
- SHA-256 校验清单

## 验收边界

R8.0 的分簇候选、人工勾选、分批发布经过代码级与模拟测试；真实 ANSYS 2024 R2 / ACT 接口、DPI 与现场 Sizing 发布仍需在实际工程机验收。本包不包含真实 ANSYS 工程、许可文件或运行时日志。
