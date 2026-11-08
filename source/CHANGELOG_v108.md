# AutoMesh v0.10.8 — Sizing 原生绑定修复

- 排查实机报错 `This property is parameterized and is read-only`；对照 ANSYS 已知的 ScopingMethod 报错。
- 创建 Sizing 时先设置 ElementSize，再优先设置 NamedSelection；仅在必要时设置 ScopingMethod。
- 必须回读并验证 ScopingMethod=Component、NamedSelection 原生关联与实际尺寸；不允许无提示退化成直接面 ID 绑定。
- 若 ACT 属性仍只读：明确报出对应 Named Selection、属性和 ANSYS 返回原因，尝试仅清理本次刚创建的无效 Sizing。
- 保留旧命名组、人工 Sizing、原有网格及原有几何，不自动清理网格或执行 GenerateMesh。
- 本版是定向修复，具体 ANSYS 2024 R2 本机行为仍需实机验收。
