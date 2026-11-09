# AutoMesh R8.1

**当前主线：R8.1 RC（2026-11-09）**。面向 ANSYS Workbench / Meshing 2024 R2 的 Windows x64 网格诊断与局部优化工具。

## 当前文件

- [Windows GUI 可执行程序](app/AutoMesh.exe)
- [R8.1 Go、Python/ACT 完整源码](source/)
- [R8.1 原始完整 ZIP 备份](releases/AutoMesh_R8_1_RC_20261109_B4E98F95E6AEB8E68B8AE5BA85E5A3Sizing_AEE6B4BAE781.zip)
- [本轮质量数据与 inlet/outlet Sizing 检查点](docs/releases/R8.1_RC_20261109_质量数据与手动出入口Sizing_检查点.md)
- [R8.1 归档校验与版本信息](docs/releases/R8.1_RC_20261109_导入核验.json)
- [SHA-256 文件校验清单](SHA256SUMS.txt)

旧 R8.0.4 发布包、失效脚本、重复文档已从当前文件树清理；过去的内容可通过 [Git 提交历史](https://github.com/1463503175-pixel/AutoMesh/commits/main/) 恢复，不再与现行 R8.1 混用。

## 功能与安全约束

R8.1 采用六页面结构：概览、连接工程、局部细化、质量报告、分析中心、设置与维护。支持 Skewness 与 Element Quality 区间标记、异常单元定位、空间分簇、局部 Sizing 候选审核、历史质量对比、来源记录和报告导出。

手工命名的 `inlet`、`outlet` 与自动识别 Named Selection 一同参与局部尺寸推荐、配置和发布；已有用户设置与历史 Sizing 必须保留。累计资源代理 120000 是**预警阈值**，不是新增单元硬上限；超阈值必须额外人工确认。仍保留几何身份、失效引用、近重合球、每批最多六个、用户审核等检查。

发布 Sizing 不得自动执行 GenerateMesh，不得未经批准删除、覆盖或 Suppress 历史 Sizing。升级时保留 `C:\ProgramData\AutoMesh2024R2\Data` 历史数据，不删除目录。

## 使用与验收

使用者需在 Windows 中打开**已有** Workbench / Meshing 会话，再由 AutoMesh 连接并选择 Mesh；网格控制的发布及后续重划都须核对当前几何。

现有检查点记录 Go / Python 定向回归和 Windows x64 GUI 交叉编译通过；**Windows 六页面实机、ANSYS ACT 原生质量采集和 inlet/outlet Sizing 发布尚待端到端验收**，不可标为正式发布成功。

仓库未附开源许可证。公开可见不等于授权他人使用或分发。
