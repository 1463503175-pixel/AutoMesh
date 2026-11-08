# AutoMesh v0.9.3 改动与验收

- 移除连接界面 mtls、wnua、insecure 选项；统一固定 localhost + insecure，仅适用用户当前本机 Workbench 连接。
- 默认连接端口为 10605；仅在端口变化时修改，成功后记住最近端口。
- 首页主按钮：未连接时直接连接；连接成功后显示「生成网格」。
- 隐藏对普通用户无帮助的 ACT 命令按钮；保留安全确认（Meshing 编辑器已打开）、只读扫描及网格保护逻辑。
- 四页适配 1100-2560 像素客户区宽度及最低 690 像素客户区高度，检查标签与按钮无交叠。
- 保留 v0.9.2 的 Workbench NoneType 返回值回退路径，防止非 JSON 响应导致直接崩溃。
- 30 项 Python 单元测试、Go 与 Go race 测试通过；Windows amd64 GUI EXE 交叉编译通过。

注意：实际 Windows 字体、DPI、ANSYS 2024 R2 的连接质量无法在此 Linux 环境中实测；预览属于源码还原，不是实机截图。
