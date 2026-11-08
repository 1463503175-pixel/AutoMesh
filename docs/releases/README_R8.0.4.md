# AutoMesh R8.0.4 · Continuous Refinement

**修复目标：** 让第一轮已经发布的历史 Sizing 在重新生成网格后继续保留、核验和计入累计预算；允许第二轮、第三轮针对当前新标记的偏度候选继续发布，不因旧网格 candidate ID 改变而进入只读锁定。

## 文件

- `AutoMesh.exe`：Windows x64 GUI 程序，ANSYS 2024 R2。
- `完整源码/`：完整 Go / Python ACT 源码、测试和操作说明。
- `完整源码/R8.0.4_连续优化与历史Sizing接管.md`：本版详细修复说明和连续优化流程。

## 本版关键改动

1. 历史 Sizing 按当前 ACT 对象 ID、Body、Sphere of Influence 类型、球心坐标系、球心、半径、尺寸和启用状态核验；不再要求旧 candidate ID 出现在新网格方案中。
2. 旧清单缺失、跨工程或损坏时，只在当前 ACT 中全部历史 QR Sizing 和坐标系均能精确读取时才允许接管；任何未知对象仍然阻断发布。
3. 历史 Sizing 不删除、不修改、不 Suppress，继续计入累计资源预算。
4. 新网格候选使用当前网格的单元 ID；旧候选 ID 不再阻断新轮次。相同候选 ID 但几何属性变化仍拒绝发布。
5. 近乎重合的球形影响域禁止重复发布；普通部分相交候选需要用户打开详情核对后再确认。
6. ACT 只读任务实时刷新历史 Sizing 清单；报告展示历史影响域；每轮发布记录独立保存。
7. 不清除网格、不执行 GenerateMesh。用户需要在 Meshing 中手动生成新网格后，再重新标记偏度、生成新方案并审核下一轮。

## 安装/升级

1. 保存当前 ANSYS 工程。
2. 关闭 AutoMesh R8.0.3 窗口；保留当前 Workbench / Meshing 会话。
3. 备份旧 AutoMesh 程序目录，使用本包 `AutoMesh.exe` 替换/启动。
4. 保留原来的 `automesh-settings.json` 和 Data 目录；不要清空任务记录或发布清单。
5. 启动后确认标题为 `R8.0.4 · Continuous Refinement`，连接现有 Workbench，打开当前 Meshing 并执行只读扫描。
6. 首次只验证历史 Sizing 是否被读取、累计预算是否显示正确、候选是否可审核；先选 1 个不重合候选试发布，不要一次性全选。

## 测试边界

- Python ACT mock：218 项通过。
- Go：全部测试通过。
- Windows x64 EXE：构建成功。
- 尚未在用户的 ANSYS 2024 R2 实际会话完成端到端验收。首次发布仍需人工检查球形影响域与晶圆片间净空。
