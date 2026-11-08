# AutoMesh v0.10.3 — 现有命名直接绑定局部 Sizing

## 核心变更

- 「局部细化」页改为可滚动的 **Meshing 现有 Named Selection 列表**，不再将面积候选列表当作局部细化对象。
- 显示原有 `AM_*` 和手工命名（如 `4inch`），自动排除 `inlet/outlet`。
- 用户选中一个命名组 → 勾选启用 → 输入局部 mm 尺寸 → 保存本组；可设置多个组，一次性发布。
- 新增 `publish_existing_sizing` 独立 ACT 任务：**仅**读取 Meshing 实时 Named Selection 的作用面，以 `GeometryDefineByType.Component` + `NamedSelection` 创建 `AutoMesh_NS_*` Sizing。
- 不在这个阶段重新选面、分组、建立 Named Selection、修改全局尺寸、删除已有网格或生成网格。
- 已有与命名组绑定的 Sizing，包含旧版生成的或手动创建的，直接复用并保留其尺寸，不创建第二套。
- 发布前验证 `inlet/outlet`、作用面 ID、命名组交叠、已有控制、当前全局尺寸和 32 GB 保守预算；存在已有网格需确认风险。
- 生成仍是独立按钮，必须通过原有审核清单，不自动覆盖网格。
- 风险预算超限、命名变更和重复 Sizing 等常见报错改为简洁中文。
- 质量报告新增“现有命名 → 实际 Sizing → 实际 mm → 面数 → 创建/保留状态”对照表。
- 精简局部细化页面，隐藏旧的四类别统一尺寸输入，修复布局测试以匹配真实可见控件。

## 离线测试和限制

- 98 项 Python 测试通过，Go 单元与竞态测试通过，Windows amd64 EXE 已编译。
- 测试使用模拟 ACT；**真实 ANSYS 2024 R2 下需实机验证 Sizing 的绑定及网格状态**。
- 保守资源预算为表面积/尺寸²代理值，不是实际网格单元数，也不等于内存实测。
