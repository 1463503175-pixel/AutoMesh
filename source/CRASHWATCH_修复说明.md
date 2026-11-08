# AutoMesh CrashWatch — R6.2 定点修复

## 可确认的根因
R6.2 只读取 Workbench/ACT 任务回执、进度 JSON，不监控 Meshing 进程。Mesh 崩溃后没有新的回执，AutoMesh 仍持续等待，且旧任务锁阻止再次提交。

## 改动
1. ACT 每次写进度时写入自身 Windows PID 和进程创建时间（FILETIME），不启动任何新进程。
2. Go 只读检查精确 PID 和创建时间。确认进程退出时停止 AutoMesh 加载，提示具体任务 ID，保留任务锁以防重复提交。
3. “检查任务状态”显示 ACT 宿主 PID 和是否仍存活。
4. 未收到 ACT 的 PID 时不假定 Meshing 已崩溃；避免将 Workbench 与 Meshing 或其他 ANSYS 会话混淆。
5. R6.2 原 Worker 哈希加入升级白名单。其余 Sizing、命名、生成、报告算法不变。

## 限制
- 这只解决已开始 ACT 任务后 Meshing 进程崩溃时的错误等待。若在 ACT 记录 PID 前崩溃，仍只能给出“任务阶段未确认”；不能冒险根据一个不确定的系统进程自动清锁。
- 不能说明 Meshing 自身崩溃根因：需要 Windows 事件查看器/可靠性监视器中的故障进程与模块，及 ANSYS 日志。
- 无 Windows + ANSYS 2024 R2 实机，离线测试不能证明现场进程监控已通过。
- 重新启动 Meshing 后，旧任务锁不会自动删除。必须人工确认旧任务终止后按原安全恢复流程操作。
