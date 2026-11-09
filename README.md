<div align="center">

<img src="docs/assets/automesh-hero.svg" width="100%" alt="AutoMesh R8.1 — an illustrated finite-element mesh with highlighted local defects" />

<br />

### Make mesh optimization measurable.

**连接已有 ANSYS 工程，找到问题单元，精准调整，验证改善。**

<p>
  <a href="app/AutoMesh.exe"><strong>Download for Windows</strong></a>
  &nbsp; · &nbsp;
  <a href="docs/GETTING_STARTED.md"><strong>快速开始</strong></a>
  &nbsp; · &nbsp;
  <a href="source/"><strong>Source code</strong></a>
</p>

<sub>R8.1 Release Candidate &nbsp; / &nbsp; Windows x64 &nbsp; / &nbsp; ANSYS Workbench &amp; Meshing 2024 R2</sub>

</div>

---

## One workflow. Better meshes.

AutoMesh 是一个面向 ANSYS Meshing 的**网格质量诊断与局部优化工作台**。它复用你已经打开的 Workbench 工程，把原本分散在质量检查、异常单元标记、Sizing 配置和历史对比中的操作，组织成连续、可核验的流程。

不重新建模，不把发布 Sizing 当作重新划分网格，也不替你做无法核验的优化结论。

```text
CONNECT          INSPECT             REFINE            VERIFY           COMPARE
连接已有工程   →   找到劣质网格单元   →   制定局部方案   →   人工审核控制   →   比较前后质量
```

## What it does

| | 能力 | 为什么有用 |
|:--:|---|---|
| **01** | **Detect defects** · 质量诊断 | 读取原生 Skewness、Element Quality 等指标，标记指定范围的异常单元，并明确记录数据来源 |
| **02** | **Refine precisely** · 局部优化 | 将异常单元分簇、识别邻近小特征、检查已有影响域，给出可人工核对的局部 Sizing 建议 |
| **03** | **Measure improvement** · 优化对比 | 保存每轮质量快照，对比最大偏度、最小及平均 Element Quality、网格数量和有来源的统计指标 |

### Two ways to refine

| **Named geometry** · 已有几何命名 | **Quality defects** · 网格缺陷驱动 |
|---|---|
| 对 `inlet`、`outlet`、晶圆面、挡板孔等 Named Selection 配置局部尺寸。 | 根据高 Skewness 或低 Element Quality 区间标记异常单元。 |
| 手动命名和自动识别都参与；入口与出口可独立设置尺寸。 | 自动分簇并形成候选，查看与历史 Sizing 的重叠和风险。 |
| **保留已有手动控制，发布前检查作用域和冲突。** | **只发布经过人工勾选、几何核验和安全检查的候选。** |

## Get started

<details open>
<summary><strong>Five steps to your first mesh review</strong></summary>

1. 在 **ANSYS Workbench 2024 R2** 中打开已有工程和目标 Meshing 编辑器。
2. 运行 [**AutoMesh.exe**](app/AutoMesh.exe)，在「连接工程」中选择当前 Mesh 系统。
3. 同步工程，在顶部点击「**更新质量**」，查看已有网格指标和数据来源。
4. 进入「局部细化」设置命名组尺寸，或在「质量报告」按偏度 / 单元质量区间标记缺陷。
5. 查看影响域、人工审核后发布 Sizing；如需重新划分网格，由你在 Meshing 中另行决定。完成重划后，在「质量分析」比较前后质量。

</details>

[**打开完整新手指南 →**](docs/GETTING_STARTED.md)

## Design principles

- **Existing session first.** 连接已有 ANSYS 工程，不自动新建模型。
- **Actual measurements.** 无有效数据时显示“未取得”；不通过偏度推算正交质量，不用假数据填充分析图。
- **Review before publish.** 保留历史 Sizing 身份校验、几何冲突检查、每批最多 6 个及人工确认。
- **No arbitrary hard stop.** 历史累计预算代理 `120000` 是预警阈值，不是实际网格单元数或内存上限；超阈值仍需额外确认。
- **No automatic remeshing.** 发布 Sizing 不自动调用 `GenerateMesh`，不擅自删除或覆盖已有控制。

## Download & resources

| 资源 | 入口 |
|---|---|
| **Windows x64 应用程序** | [Download AutoMesh.exe](app/AutoMesh.exe) |
| **完整源码** | [Go + Python + Meshing ACT](source/) |
| **新手使用指南** | [Getting started](docs/GETTING_STARTED.md) |
| **本次完整发布包** | [R8.1 RC ZIP](releases/AutoMesh_R8_1_RC_20261109_B4E98F95E6AEB8E68B8AE5BA85E5A3Sizing_AEE6B4BAE781.zip) |
| **测试记录与已知限制** | [Development checkpoint](docs/releases/R8.1_RC_20261109_质量数据与手动出入口Sizing_检查点.md) |
| **文件完整性** | [SHA-256 manifest](SHA256SUMS.txt) |

<details>
<summary><strong>Architecture · 技术实现</strong></summary>

```text
AutoMesh.exe            Go · Native Windows UI
       │
       ▼
Python / PyWorkbench    连接现有 Workbench 脚本服务
       │
       ▼
Meshing ACT Worker      质量采集、标记、局部方案、安全检查
       │
       ▼
ANSYS Named Selection / Sizing
       │
       ▼
Task receipts · Quality history · Reports
```

</details>

<details>
<summary><strong>Upgrade notes · 升级须知</strong></summary>

关闭旧版程序后替换 EXE。请保留 `C:\ProgramData\AutoMesh2024R2\Data`；这里可能保存任务回执、质量快照和历史 Sizing 记录，不应为了升级而删除整个文件夹。

</details>

---

<div align="center">

<sub><strong>Release candidate, not a certified ANSYS workflow.</strong> Windows 六页面实际交互、ANSYS ACT 原生质量指标以及 inlet/outlet Sizing 仍有实机待验收项目。以上网格主视觉为原创技术示意图，不是实际 ANSYS 计算截图。</sub>

<br />

<sub>项目暂未附开源许可证。历史版本可通过 <a href="https://github.com/1463503175-pixel/AutoMesh/commits/main/">Git history</a> 查找。</sub>

</div>
