# AutoMesh v0.9.7 — 可编辑几何分组与局部网格控制

## 本轮功能（基于 v0.9.6 原工程增量修改）

- 新增安全的 `prepare` 阶段：**只创建 Named Selection 和关联 Sizing，不执行 GenerateMesh**；发布完成后在 Meshing 模型树中选取、高亮、增减作用面、修改尺寸。
- 新增独立 `generate_reviewed` 阶段：只读取已发布/人工修订的控制；检查实际 Named Selection / Sizing 绑定、几何指纹、边界、单位、狭缝最小分辨率和 32 GB 面积代理预算；随后由用户确认，调用一次 GenerateMesh。
- 已验证圆柱环面按孔径带、所属体和大范围坐标分段；前述分组**不是**孔洞通透性或具体挡板身份判定。
- 已识别的球面/锥面/环面作为低置信度解析曲面组，默认不启用；自由曲面不自动施加错误网格尺寸。
- `AM_*` 名字出现在原生 Meshing Named Selections，`AutoMesh_*` 原生 Sizing 指向对应 Named Selection；额外的 AM_* 默认 `SendToSolver=False`，保留 inlet/outlet 自身不修改。
- 重复发布不覆盖任何现有同名 Named Selection 的作用面或同名 Sizing 的用户尺寸；若旧版控制存在或不能验证绑定，会中止并说明；不自动清理用户控制。
- 全局 ElementSize / MaximumSize / GrowthRate 仅在明确勾选覆盖全局后修改。
- 现有 Workbench、Meshing 会话保留，不执行 Fluent、不重启软件、不截图坐标控制。
- 面积/尺寸²只用于保守否决，不承诺实际单元数、32 GB RAM 消耗或 CFD 收敛。

## 需要真实 ANSYS 2024 R2 验收的关键点

1. Workbench 对当前打开的 FFF Meshing 的命令路由确实落在原窗口。
2. ACT `Model.AddNamedSelection`、`NamedSelection.SendToSolver`、`GeometryDefineByType.Component`、`Sizing.NamedSelection` 在当前许可与 Meshing 类型下工作，并在模型树中可见。
3. 可手工编辑对象后再次执行 `prepare` 能保留修改。
4. 生成网格后核查最大偏度、最小正交质量、总单元数与实际片间流道覆盖。

这些实机项目验收尚未完成。离线 Python/Go 测试仅证明本机模拟接口与逻辑一致性，不代表已经在 FFF 中成功生成网格。
