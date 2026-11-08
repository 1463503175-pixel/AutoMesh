# v1.0.9 / R8.0.2

- 修复 IronPython ACT 终态 JSON 回执写入：使用 Win32 MoveFileExW 和可恢复 rename 兜底。
- 局部预览方案携带本次 Job ID；仅在 Job ID、工程和几何指纹全部匹配时恢复成功。
- 任务轮询期间可恢复 ACT 明确终态标记，不再必须重启 AutoMesh 才处理回执故障。
- 增加局部方案预览阶段进度提示；预览仍只读，不触碰 Mesh。
- 跨工程旧 QR 清单仅在当前 Mesh 树仍有 QR 控件时阻断；无 live 控件时隔离旧清单，避免全局 Data 残留误伤新工程。
- 「任务状态」按钮可在安全等待期后恢复明确的 ACT 终态标记，不触碰 ANSYS。
