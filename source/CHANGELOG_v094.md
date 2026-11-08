# AutoMesh v0.9.4 热修复

- 修复 2024 R2 + PyWorkbench 0.14.0 返回普通文本 `AUTOMESH_PROBE_OK` 时被误解析为 JSON 的连接失败。
- 连接时直接执行已有 Workbench 的只读系统列表查询，不再使用有兼容性问题的前置探针。
- 仍保留 `None` 返回值本地 JSON 文件回退；绝不新建 Workbench/Meshing 或执行求解。
- 补充真实报错模拟：裸探针文本、JSON 引号字符串、`FFF` 系统列表。
- 保留 v0.9.3 的四页布局和快速重绘修复。

## 用户操作
关闭旧版 AutoMesh，解压 v0.9.4 并启动，保持 Workbench `StartServer()` 端口 10605，不要再次 StartServer，不要重新安装 Python。按连接按钮后应列出 FFF；若仍失败，检查 `%ProgramData%\AutoMesh2024R2\Data\last_workbench_error.log`。

## 验收边界
离线回归和 Windows EXE 交叉编译通过不等于在用户本机 ANSYS 2024 R2 上连接成功。
