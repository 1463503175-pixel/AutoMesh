<div align="center">

<br>

<img src="docs/media/icon.png" width="96" alt="AutoMesh">

<h1>AutoMesh</h1>

<h3>Better meshes. Better engineering decisions.</h3>

<p><strong>让每一次网格优化，都有据可循。</strong></p>

<p>专为 ANSYS Meshing 打造的网格质量诊断与局部优化工作站。<br>
连接已有工程，从真实质量数据出发，将网格缺陷转化为精准、可审核的优化决策。</p>

<a href="https://github.com/1463503175-pixel/AutoMesh/raw/main/app/AutoMesh.exe"><img alt="Download for Windows" src="https://img.shields.io/badge/Download%20for%20Windows-0866D9?style=for-the-badge&amp;logo=windows&amp;logoColor=white"></a>
<a href="https://github.com/1463503175-pixel/AutoMesh/raw/main/AutoMesh_R8_1_BatchHotspot_20261110_Full.zip"><img alt="Full source package" src="https://img.shields.io/badge/Full%20source%20package-24292F?style=for-the-badge"></a>

<br>

<a href="docs/GETTING_STARTED.md">快速开始</a> &nbsp;·&nbsp; <a href="source/">完整源码</a> &nbsp;·&nbsp; <a href="source/发布说明/R8.1.2_自动开启Workbench服务与连接.md">版本说明</a> &nbsp;·&nbsp; <a href="https://github.com/1463503175-pixel/AutoMesh/issues">问题反馈</a>

<sub>R8.1.2 RC &nbsp; / &nbsp; Windows x64 &nbsp; / &nbsp; ANSYS 2024 R2 &nbsp; / &nbsp; Native desktop</sub>

<br><br>

</div>

<img src="docs/media/brand.svg" width="100%" alt="Beyond mesh generation. Engineer mesh quality.">

## Beyond mesh generation.

**网格划分完成，不代表网格优化完成。**

AutoMesh 将质量采集、异常单元定位、局部尺寸控制和优化前后对比组织成连续的工作流。它面向已有 ANSYS 工程，不要求重复建模，也不把发布 Sizing 当作重新划分网格。用真实数据发现问题、审核调整并衡量改善。

## What's new in R8.1.2

- **智能连接：** 优先复用现有 Workbench 脚本服务；在唯一识别的现有 Workbench 中，必要时按可访问性对象启动服务并验证实际网格系统。
- **大规模热点分析：** 分批处理异常单元，检查处理数量及候选覆盖率；11,671 个异常单元的合成回归案例验证了完整预览与受控发布逻辑。
- **稳定的桌面会话：** 增加托盘恢复、单实例窗口恢复与生命周期记录。
- **安全升级：** 校验已知官方组件身份，保护用户自定义脚本及未结束任务。

详见 [R8.1.2 原始发布与验收记录](source/发布说明/R8.1.2_自动开启Workbench服务与连接.md)。

## What you can do

<table>
<tr>
<td width="50%" valign="top">
<h3>01 &nbsp; Quality intelligence</h3>
<b>看清真正的网格质量</b>
<p>读取可取得的原生 Skewness、Element Quality 和正交质量数据，按指定范围筛选与标记异常，记录指标来源。</p>
</td>
<td width="50%" valign="top">
<h3>02 &nbsp; Spatial refinement</h3>
<b>将优化集中在问题区域</b>
<p>从命名几何或异常单元分簇生成局部 Sizing 候选，核对作用域、覆盖情况与既有控制。</p>
</td>
</tr>
<tr>
<td valign="top">
<h3>03 &nbsp; Controlled publishing</h3>
<b>让每一步调整都可核验</b>
<p>预览与发布分离；局部控制经过审核和安全检查后才写入，不擅自重新划分网格。</p>
</td>
<td valign="top">
<h3>04 &nbsp; Quality analytics</h3>
<b>用数据证明优化效果</b>
<p>对照优化前后的质量统计、网格数量和历史快照，保留报告与任务记录。</p>
</td>
</tr>
</table>

## One workflow. Better meshes.

| 阶段 | 操作 |
|:--|:--|
| **01 — Connect** | 打开已有 Workbench 工程，通过 AutoMesh 连接目标 Meshing。 |
| **02 — Inspect** | 获取原生质量指标，定位需要处理的异常单元。 |
| **03 — Refine** | 按实际几何和空间缺陷生成局部尺寸优化方案。 |
| **04 — Review** | 核对覆盖及冲突，人工确认后发布 Sizing。 |
| **05 — Compare** | 重新划分网格后读取新质量数据，比较优化效果。 |

## Get started

下载最新 Windows 程序，打开已有 ANSYS Workbench 工程及目标 Meshing 编辑器，再运行 AutoMesh。优先复用实际工程和既有服务。

**[Download AutoMesh R8.1.2](https://github.com/1463503175-pixel/AutoMesh/raw/main/app/AutoMesh.exe)** &nbsp; · &nbsp; [完整源码包](https://github.com/1463503175-pixel/AutoMesh/raw/main/AutoMesh_R8_1_BatchHotspot_20261110_Full.zip) &nbsp; · &nbsp; [完整使用指南](docs/GETTING_STARTED.md)

## Explore the project

[Desktop application](app/) &nbsp; · &nbsp; [Go + Python source](source/) &nbsp; · &nbsp; [Build entry](source/build_windows_gui.py) &nbsp; · &nbsp; [Documentation](docs/)

<details>
<summary><strong>Release integrity &amp; engineering scope</strong></summary>

当前主分支以 2026-11-22 上传的完整压缩包为唯一应用来源。实际版本为 R8.1.2 RC。已恢复 179 个原始项目文件，上传包原自带的 178 项校验全部一致。

- [文件 SHA-256](SHA256SUMS.txt)
- [上传包核验清单](docs/release_inventory.json)
- [原版发布说明与实机记录](source/发布说明/R8.1.2_自动开启Workbench服务与连接.md)

原版发布说明记录 275 项 Python 测试通过、223 项 Go 测试通过、3 项跳过。本次仓库同步核对的是原始打包字节、源码语法及文件完整性，未重新执行完整回归或 ANSYS 实机测试。已知不同网格工作流和多个 Workbench 会话仍受实际环境限制。

</details>

---

<div align="center">

<sub>AutoMesh &nbsp;·&nbsp; Precision refinement. Traceable quality.</sub>

</div>
