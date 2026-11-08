# v0.9.5
- 新增「局部细化」页面：显示三类候选的建议尺寸/涉及面数/置信度，支持启用/关闭及 mm 尺寸覆盖。
- 必须先完成本次系统的只读扫描再生成；删除界面直通 auto_onepass 的路径，改用有几何指纹及 Named Selections 复核的 auto 任务。
- 自动生成前验证 inlet、outlet 命名面存在、不重叠且属于当前几何；不自动设置 Fluent 工况。
- 从 `ActiveUnitSystem` 读取 Meshing 显示单位，与 `GeoData.Unit` 底层坐标单位分开展示；几何尺寸换算逻辑仍使用已验证单位系数。
- 自定义蓝色 AutoMesh 图标打包为 Windows PE RT_ICON/RT_GROUP_ICON，并用于窗口/任务栏。
- 扩展扫描和 HTML 质量报告：局部建议表、执行记录、扫描警告及两种单位。
- 单元测试覆盖命名边界缺失、局部尺寸覆盖/跳过、几何单位 m 到 mm 转换。
- 不保证真实 ANSYS 会话的 meshing 行为，自动细化不是物理通道识别的证明。
