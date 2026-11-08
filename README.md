# AutoMesh R8.0.4

**面向 ANSYS Workbench / Meshing 2024 R2 的网格质量检查与局部细化辅助工具。**

AutoMesh 连接用户已经打开的 Workbench 工程和 Meshing 编辑器，读取几何、Named Selection、网格参数、质量指标及已有 Sizing；再将可审核的局部细化建议转成 ANSYS 原生网格控制。它的目标不是替代 Meshing，而是让网格问题定位、局部控制发布和多轮优化更有记录、更可核验。

## 快速入口

- Windows x64 程序：[app/AutoMesh.exe](app/AutoMesh.exe)
- R8.0.4 完整发布包：[releases/AutoMesh_R8_0_4_Continuous_Refinement.zip](releases/AutoMesh_R8_0_4_Continuous_Refinement.zip)
- **第一次使用请先阅读：[AutoMesh R8.0.4 新手使用手册](docs/USER_GUIDE_R8.0.4.md)**
- R8.0.4 版本说明：[docs/releases/R8.0.4_连续优化与历史Sizing接管.md](docs/releases/R8.0.4_连续优化与历史Sizing接管.md)
- 完整源码：[source/](source/)

## 1. AutoMesh 能做什么

- **连接现有工程：** 复用已经打开的 Workbench / Meshing 会话，不另起工程。
- **几何只读扫描：** 读取 Body、Face、面积、面型、单位、Named Selection、全局网格参数和已有 Mesh 控件。
- **质量检查：** 读取 ACT 当前会话实际开放的偏度、正交质量等指标；读取失败或接口未开放时明确标记，不用估算值替代。
- **局部细化建议：** 支持基于现有 Named Selection 的尺寸推荐与分组，也支持从网格质量警告单元生成空间分簇和球形影响域候选。
- **可审核发布：** 将明确勾选的方案发布为 ANSYS 原生 Named Selection / Sizing / Sphere of Influence Sizing。
- **连续优化：** R8.0.4 将历史 Sizing 身份与当前网格候选身份分离，重新划分网格后仍可核验并保留历史控制，继续审核新一轮候选。

## 2. 两种局部细化方式

| 方式 | 输入对象 | 主要用途 | 发布结果 |
|---|---|---|---|
| 现有 Named Selection 细化 | Meshing 中已有的几何命名组 | 对已命名的孔、晶圆面、间隙、挡板等区域设置尺寸 | 绑定到 Named Selection 的原生 Sizing |
| 质量警告区细化 | ANSYS Worksheet 标记的网格元素 / 指定偏度区间 | 对高偏度或警告单元定位、分簇并形成局部影响域 | 球形 Sphere of Influence Sizing + 对应坐标系 |

两种方式都需要人工核对作用范围。警告区路线不会自动判断晶圆片间净空；影响域报告会明确要求使用者结合真实几何逐项检查。

## 3. 第一次使用：最短流程

1. 打开 ANSYS Workbench 2024 R2 的现有工程，并双击目标系统的 Mesh 单元，打开 Meshing 编辑器。
2. 启动 AutoMesh，进入“连接工程”，点击“自动查找并连接”。
3. 选择目标 Mesh 系统，勾选“我已在 Workbench 中打开目标 Meshing 编辑器”。
4. 回到“概览”，点击“只读扫描”。
5. 根据目标选择路线：
   - 已有 Named Selection：进入“局部细化”，检查命名组、启用状态和尺寸，再发布 Sizing。
   - 已有质量问题：进入“质量报告”，先检查质量，再标记警告 / 偏度、生成局部方案并逐项审核。
6. 发布后回到 Meshing 树检查真实控件。发布 Sizing 不会自动生成网格；需要重划时必须单独确认。
7. 重新生成网格后，对当前网格重新执行质量检查。旧网格元素 ID 不可直接复用。

## 4. 五个页面怎么理解

| 页面 | 主要任务 | 新手先做什么 |
|---|---|---|
| 概览 | 查看当前工程、几何规模、单位、网格参数、任务和质量摘要 | 确认工程后点击“只读扫描” |
| 连接工程 | 连接现有 Workbench、选择 Mesh 系统、确认编辑器已打开 | 自动连接 → 选系统 → 勾选确认 |
| 局部细化 | 管理 Named Selection 分组、尺寸、启用状态和 Sizing 发布 | 刷新命名 → 检查作用面 → 设置尺寸 |
| 质量报告 | 只读质量检查、失败 / 警告 / 偏度标记、局部方案预览 | 先质量检查，再标记和预览 |
| 更多设置 | 导出 Fluent .msh、迁移工作数据目录 | 仅在需要导出或迁移时使用 |

质量报告中的“局部优化候选”是独立审核页面：默认不勾选候选，必须打开影响域详情、核对几何和片间净空后，才能确认发布。

## 5. 软件内部如何工作

AutoMesh 由 Go 原生 Windows 桌面界面、Python / PyWorkbench 连接桥和运行在当前 Meshing 会话内的 ACT / IronPython Worker 组成。

~~~text
AutoMesh.exe
   │ 任务 ID / JSON 请求 / 状态锁
   ▼
Python + PyWorkbench
   │ 连接本机现有 Workbench 脚本服务
   ▼
选定的 Workbench Mesh 系统
   ▼
Meshing ACT Worker
   │ 只读扫描 / 质量检查 / 标记 / 分簇 / 安全核验
   ▼
ANSYS 原生 Named Selection / Sizing
   │
   ▼
JSON 结果、进度记录和 HTML 报告
~~~

关键机制：

1. **工程身份核验：** 扫描 Body、Face、单位、命名和网格参数，生成几何指纹；后续任务重新核验，防止旧方案误用于变化后的模型。
2. **几何推荐：** 根据命名组实际作用面、面积中位数、面型和参考尺寸形成启发式建议；面积和名称不能替代物理判断。
3. **警告单元分簇：** 读取当前标记单元的坐标、节点和 Body 归属，按连通关系与空间邻近关系拆分为多个小影响域。
4. **历史控制核验：** 发布前重新读取现有 Sizing、作用域、坐标系、球心、半径和尺寸；不以旧网格候选 ID 代替历史控件身份。
5. **分批提交：** 只发布用户勾选且通过校验的候选；写入后回读 ANSYS 控件属性，核对与审核方案一致。
6. **任务保护：** 每个任务有独立 ID 和状态记录；远端任务是否结束未确认时保留任务锁，防止重复提交。

R8.0.4 警告区路线的关键限制：单簇最多 64 个标记单元、球半径不超过 12 mm、目标尺寸 0.5–5 mm、单次分析最多 4096 个标记单元、每批最多发布 6 个、历史与新增球形控制的累计预算代理上限为 120000。预算代理不是实际网格单元数或精确内存预测。

## 6. 重要安全边界

- 连接、只读扫描、只读质量检查、风险抽样和方案预览不会生成网格。
- 标记失败 / 警告 / 偏度会在 Meshing 命名选择树中创建元素选择组，但不清除网格。
- 发布 Sizing 会改变 Meshing 控制树；已有网格可能因此失效，但发布动作本身不会自动清网格或调用 GenerateMesh。
- 只有明确进入“审核后生成网格”并通过确认的受控流程，才可能清除旧网格并重新生成。执行前必须保存工程。
- 警告区 Sizing 发布后，仍需在 Meshing 树核对并单独决定何时重新划分。
- 导出 .msh 不是 Fluent 质量验收；边界、正交质量和 CFD 网格拓扑仍需复核。
- “请求停止任务”是协作式停止，不会强制结束 ANSYS 进程；状态未知时不要再次提交或擅自清除任务锁。

## 7. 仓库结构

| 目录 / 文件 | 内容 |
|---|---|
| **app/** | Windows x64 可执行程序 AutoMesh.exe |
| **source/** | Go 主程序、Python / ACT Worker、Workbench 桥接、自动化脚本和测试 |
| **docs/** | 新手手册、R8.0.4 版本说明和归档文档 |
| **releases/** | 集成发布包、原始 ZIP 和发布归档 |
| **tools/legacy/** | 历史发布脚本，不是日常运行入口 |
| **SHA256SUMS.txt** | 当前整理后仓库文件的 SHA-256 校验清单 |

## 8. 运行要求与验收状态

- Windows x64；
- ANSYS Workbench / Meshing 2024 R2；
- Python 3 环境可导入 **ansys.workbench.core**；
- 使用现有工程和已经打开的 Meshing 编辑器。

R8.0.4 版本说明记录：Python 离线测试 218 项通过、Go 测试全部通过、Windows x64 EXE 构建成功。**离线测试和模拟 ACT 测试不等同于在用户实际 ANSYS 2024 R2 会话完成端到端验收。** 首次使用请先做小范围验证，不要直接批量发布或重划大型现有网格。

## 9. 文档入口

- [新手使用手册：页面、按钮、操作流程、内部逻辑和故障排查](docs/USER_GUIDE_R8.0.4.md)
- [R8.0.4 连续优化与历史 Sizing 接管说明](docs/releases/R8.0.4_连续优化与历史Sizing接管.md)
- [完整源码](source/)
- [完整发布包](releases/AutoMesh_R8_0_4_Continuous_Refinement.zip)
- [SHA-256 校验清单](SHA256SUMS.txt)

## 10. 许可证说明

仓库目前未附带开源许可证。公开可见不等于授予他人任意修改、再分发或商用的许可。
