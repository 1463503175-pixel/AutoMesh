# AutoMesh R8.1

**当前版本：R8.1 RC（2026-11-09）**。面向 ANSYS Workbench / Meshing 2024 R2 的 Windows x64 原生网格诊断与局部细化辅助工具。

此仓库的当前可运行候选版已从上传的 R8.1 源码 ZIP 校验并展开到现有目录，保留 Git 提交历史及 R8.0.4 历史发布包。

## 当前版本入口

- [Windows x64 GUI 程序](app/AutoMesh.exe)
- [R8.1 完整源码（Go + Python + ACT）](source/)
- [R8.1 本轮检查点及测试证据](docs/releases/R8.1_RC_20261109_质量数据与手动出入口Sizing_检查点.md)
- [本轮导入完整性核验](docs/releases/R8.1_RC_20261109_导入核验.json)
- [仓库文件 SHA-256 清单](SHA256SUMS.txt)
- [R8.0.4 历史使用手册](docs/USER_GUIDE_R8.0.4.md)（旧版资料）

本次上传的原始 R8.1 完整 ZIP 保留在仓库根目录，未复制成第二份发布包。

## 功能简介

R8.1 采用六页面结构：**概览、连接工程、局部细化、质量报告、分析中心、设置与维护**。

- 连接已经打开的 Workbench 工程与 Meshing 编辑器；跟踪工程和 Mesh 身份、共享质量状态及任务回执。
- 支持 **Skewness 偏度**、**Element Quality 单元质量**的自定义区间标记，以及局部缺陷分簇、影响域检查和人工审核。
- 已将手动命名的 **inlet / outlet** 等 Named Selection 接入局部细化推荐与 Sizing 发布流程；不强制覆盖已有控制，不统一硬编码出入口尺寸。
- 历史 Sizing 保留对象身份与跨网格轮次记录，检查重复作用域和几何身份；每批最多 6 个并要求人工确认。
- 旧的累计预算 120000 作为预警阈值；超阈值发布仍需要额外人工确认及其他安全校验。
- 分析中心提供质量快照、历史趋势、优化前后比较及报告导出；缺失真实指标时明确标注数据来源与局限。
- 受管理 Python 脚本可按安装回执进行升级；不要通过删除整个 C 盘 Data 目录来升级。

## 使用方式

1. 在 Windows 中打开已有的 Workbench 2024 R2 工程和目标 Meshing 编辑器。
2. 启动 [AutoMesh.exe](app/AutoMesh.exe)，连接现有会话，确认目标 Mesh。
3. 根据需要进行完整几何扫描、质量检测、命名组局部细化或异常单元标记。
4. 对候选 Sizing 查看真实影响域与已有控制，人工确认后发布。
5. 是否重新划分网格由使用者在 Meshing 中单独决定；发布 Sizing 不等于自动生成网格。

**重要限制：** AutoMesh 不应自动执行 GenerateMesh，不应未经确认删除、覆盖或 Suppress 已有 Sizing。累计预算代理不是实际新增网格单元数，也不是内存使用量。

## 验收状态

本次 R8.1 ZIP 来源及 SHA-256 已校验：248 个归档条目、247 条文件哈希完全匹配；提取 221 个源码文件，排除 24 个 Python 编译缓存。GUI EXE 为 Windows x64 候选版。

检查点记录了 Go、Python 的相关定向测试、Go Vet 与 Windows GUI 交叉编译通过。**仍未完成 Windows 原生六页面实机测试、ANSYS 2024 R2 ACT Worksheet 质量采集和 inlet/outlet Sizing 端到端验收。** 不能将候选版称为已验收的最终正式版。

## 仓库布局

| 位置 | 用途 |
|---|---|
| `app/` | 最新 R8.1 Windows GUI EXE |
| `source/` | R8.1 Go、Python Worker、ACT 脚本、资源与测试 |
| `docs/releases/` | 当前 R8.1 检查点、历史 R8.0.4 文档 |
| `releases/` | 保留的历史发布归档 |
| `tools/` | 历史工具 |
| `.github/workflows/` | 仓库已有发布流程与 R8.1 归档导入记录 |

原来的 R8.0.4 源码仍可通过 Git 历史和 R8.0.4 归档获取，没有被当作当前 R8.1 主线。

## 许可说明

当前仓库未包含开源许可证。公开可见不等于自动获得使用、修改、再分发或商用授权。
