# AutoMesh R8.1.2 · Quick start

## 01 / 下载与启动

下载 [Windows EXE](https://github.com/1463503175-pixel/AutoMesh/releases/download/R8.1.2/AutoMesh.exe) 或 [完整源码包](https://github.com/1463503175-pixel/AutoMesh/releases/download/R8.1.2/AutoMesh_R8_1_BatchHotspot_20261110_Full.zip)。程序为原生 Windows x64 应用，原包注明 ANSYS Workbench 2024 R2。

先打开已有 Workbench 工程及需要的 Meshing 编辑器，再运行 AutoMesh。软件优先验证已有脚本服务；若未开启，且能唯一识别已有 Workbench，会尝试启动服务并核对实际系统。多个 Workbench 会话须消除歧义。不会新建 Workbench 进程或修改工程来完成连接。

## 02 / 质量检查和优化

1. 连接目标 Mesh 系统，读取真实已有网格质量。
2. 对命名几何调整局部尺寸，或按质量范围标记异常单元。
3. 查看局部细化候选及覆盖与冲突检查，审核后发布。
4. 需要重划网格时，在 Meshing 中自行决定；完成后读取新质量并对比。

对于大量异常单元，当前版本支持分批处理及完整性核验。原包的 11,671 个异常单元测试是合成回归，不是实际工程网格优化验收。

## 03 / 更新和源码

关闭旧 AM 后进行版本替换。保留本地配置、任务历史和数据目录，不为升级删除全部数据。

[完整 Go/Python 源码](../source/) | [Windows GUI 构建脚本](../source/build_windows_gui.py)

使用 Go 1.23+，在 source 目录运行：

```shell
python build_windows_gui.py
```

构建脚本将输出新的 AutoMesh.exe 到 source 的上级目录；已发布的 app/AutoMesh.exe 保持原始上传字节。

## 04 / 验收和限制

[原版 R8.1.2 发布说明](../source/发布说明/R8.1.2_自动开启Workbench服务与连接.md) · [文件核验记录](release_inventory.json)

原版记录显示当前 2024 R2 单一 Workbench 会话连接通过，275 项 Python 测试通过，223 项 Go 测试通过、3 项跳过。未重启 WB 进行第二轮冷启动，也没有执行网格划分或求解。本次仓库同步没有重新完成这些实机操作。
