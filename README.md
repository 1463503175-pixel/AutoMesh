# AutoMesh R8.0

ANSYS Meshing 网格质量检查与局部细化工具。

## 仓库结构

| 目录/文件 | 内容 |
|---|---|
| `app/` | Windows x64 可执行程序 `AutoMesh.exe` |
| `source/` | Go 主程序、Python/ANSYS Worker、自动化脚本、测试与历史技术记录 |
| `docs/` | R8.0 发布说明及上传归档文档 |
| `releases/` | 集成发布包、原始 ZIP 及原始校验清单 |
| `tools/legacy/` | 首次发布时使用的历史上传脚本 |
| `SHA256SUMS.txt` | 当前整理后仓库文件的 SHA-256 校验清单 |

## 下载与运行

- Windows 程序：[`app/AutoMesh.exe`](app/AutoMesh.exe)
- 原始集成发布包：[`releases/AutoMesh_R8_0_FullCandidate_ManualSizing_Integrated.zip`](releases/AutoMesh_R8_0_FullCandidate_ManualSizing_Integrated.zip)
- 原始完整上传 ZIP：[`releases/archives/AutoMesh_R8_0_GitHub_Ready.zip`](releases/archives/AutoMesh_R8_0_GitHub_Ready.zip)
- 发布说明：[`docs/releases/R8.0_候选审核与增量发布说明.md`](docs/releases/R8.0_候选审核与增量发布说明.md)

## 源码测试

在 Windows 开发环境进入 `source/` 后执行：

```powershell
go test ./...
python -m pytest -q
```

测试结果应结合 ANSYS 2024 R2 / ACT 实际环境单独验收。代码级或模拟测试不等同于真实 Meshing Sizing 发布验收。

## 安全边界

AutoMesh 面向用户已经打开的 Workbench/Meshing 会话；不会为了测试而新建工程、自动重划网格或启动 Fluent。公开仓库不包含真实工程、ANSYS 许可文件或个人凭据。

仓库未附带开源许可证，因此公开可见不代表授予他人任意再分发或修改的许可。
