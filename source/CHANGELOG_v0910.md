# AutoMesh v0.9.10 — 已有网格独立命名与细化发布

## 本轮解决的实际报错

`RuntimeError: Mesh already generated; refuse to add controls to a meshed project`

此报错由 v0.9.9 把「创建 Named Selection」与「创建 Sizing」捆绑且对已有网格一律拒绝导致。

## 主要调整

- 新增 **仅发布命名**：已有网格时仍可创建 `AM_*` Named Selection；原有 `inlet/outlet` 保留，人工修改的已有同名作用域保留；新建 `AM_*` 默认不传入 Fluent。
- 单独 **发布局部 Sizing**：已有网格时需风险二次确认；工作端再次核对 `allow_existing_mesh_sizing`，不自动执行 `ClearGeneratedData` 或 `GenerateMesh`，且不允许此时修改全局尺寸。
- **审核后生成网格**：仍拒绝覆盖当前已生成网格，只有网格不存在且安全校验通过才执行。
- 仅发布命名后要求重新只读扫描，防止使用创建前的旧 Named Selection 快照。
- 返回报告附带发布前/后的单元数量和是否保持不变；新增针对已有网格的三项模拟回归测试及独立按钮布局测试。
- 主界面更新为三个并列按钮，不显示原始 Traceback 作为按钮操作说明。

## 已知限制

本环境无法访问用户当前 ANSYS 2024 R2 Meshing。ACT `Model.AddNamedSelection()` 在已生成网格时能否保持网格有效仍需在实际模型验证；程序会检测单元数量变化，但不能阻止 ANSYS 自身因对象变化而将已有网格标记为过期。需要保留现有结果时务必先保存工程及数据备份。

## 验证

81 项 Python 测试通过、Go 单元及 race 测试通过、Windows x64 GUI EXE 编译成功。此为模拟接口与编译验证，不是 ANSYS 实机验收。
