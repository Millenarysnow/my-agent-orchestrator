# 本地 Orchestrator 替换与进程生命周期

本文约定本地项目切换 Orchestrator harness 时，旧会话、agent 进程和 Git worktree 如何交接。通用会话生命周期见 [architecture.md](../architecture.md)，项目分支与 worktree 基线见 [本地项目基线与远端解耦.md](本地项目基线与远端解耦.md)。

## 目标与非目标

**替换成功后，旧 Orchestrator 的 controller/provider host 已停止，旧 worktree 已释放，新 Orchestrator 才能使用项目的编排器分支。** UI 的 `terminated` 事实不能替代操作系统进程已经退出或 worktree 已释放的确认。

**停止失败或 worktree 未释放时，替换必须失败并保留可恢复事实。** 不允许把未确认停止的 provider 留在后台，也不允许在旧目录仍被占用时创建新 session 来碰运气。

**本方案不改变项目配置的保存语义、Agent 选择和 TUI/Chat 能力判定。** 设置保存后即使替换失败，已保存的 harness 仍是后续新会话的配置；错误必须表达替换阶段失败，而不是暗示配置没有保存。

本方案不改变普通 Worker 的启动、用户显式 Kill、正常 daemon 关闭时保留 Chat host 的语义，也不向 AO 增加按 PID 强杀 provider 的第二套管理路径。

## 核心方案

**替换是一段由 daemon 串行拥有的退休事务：停止旧 controller，保存未提交工作，记录不可恢复旧 session 的终止事实，执行已接受的工作区清理，最后启动新 session。** 该方案复用 Chat service、session manager、项目级替换锁和现有 `session_cleanup_facts`，因为这些边界分别拥有 provider、session/worktree 生命周期和可重试清理状态。

## 结构与依赖边界

**`service/session.Service.SpawnOrchestrator` 拥有项目级协调。** 它在项目锁内先处理该项目此前已接受但尚未释放的 Orchestrator worktree，再退休当前非终止 Orchestrator，最后才调用 Spawn。锁跨越停止、清理和新 Spawn；这样其它 Orchestrator 请求不能同时接管同一个规范分支。

**`session_manager.Manager` 拥有旧 session 的停止与目录释放。** Chat 会话必须经 `ChatLauncher.StopChat` 停止；TUI 会话必须经 runtime adapter 销毁其已记录的 runtime。两种模式都由同一个 session operation guard 排斥输入、恢复和其它独占操作。

**`service/chat.Service` 和 persistent host 仍是 Chat 进程所有者。** 替换只能调用 Stop 并等待 controller/host 的退出确认；不能在 session manager、HTTP controller 或 renderer 中另行按 PID 杀进程。daemon 正常关闭仍可 detach host，以保留正在执行的 Chat 工作。

**工作区清理复用 `session_cleanup_facts`。** 替换在确认旧 controller 停止后，可以接受强制释放这个专用 Orchestrator worktree；若释放失败，使用明确标记的 `pending` 清理事实。其它 Kill、自动 Cleanup 和 dirty-worktree 保护保持原语义。

## 替换顺序与失败语义

**替换前不向旧 agent 发送“即将被替换”消息。** 这类文本通过普通 Chat 消息通道，会启动一轮真实模型调用；它既不能证明 provider 已退出，也不能代替 Stop。旧 controller 的停止是本地控制面操作，不是模型任务。

**先取消旧 session 尚未完成的异步 Chat spawn，再取得 session operation guard 并读取最新记录。** 若 provisioning 与替换并发，只有先等待/取消 provisioning，才能确定是否已有 provider host 需要停止；否则新 controller 可能在替换检查通过后才启动。

**Chat Stop 必须成功返回后，才可继续退休。** Stop 包括关闭 controller 和终止持久 provider host。Stop 报错或超时表示退出未确认：旧 session 不标记终止，worktree 不修改，也不启动替代 session。

**确认 controller 停止后再关闭 scoped shell、导入附件和保存未提交 worktree 内容。** agent 仍运行时先做 Git 快照存在并发写入窗口；快照之后落盘的文件会丢失。只有内容成功保全，才可清理 worktree。

**终止记录必须先于目录释放，且仅在旧 agent 已停止后写入。** 这阻止 daemon 重启恢复旧 Chat/TUI session 并重新拉起旧 agent；若随后文件系统删除失败，旧 session 保持 terminated，同时其 worktree 清理状态为 pending。

**pending 的替换清理保留“强制退休”语义，由 daemon 启动恢复和下一次创建/替换该项目 Orchestrator 前重试。** 该事实只会由已经确认停止、已经保存 worktree 内容的替换路径写入，因此重试可以继续释放该目录；普通的 dirty cleanup 仍不得被自动改成强删。若重试仍不能释放目录，新的 Spawn 不得开始。

**工作区释放失败只返回不含绝对路径或 OS 原始错误的明确 API 错误。** `Unable to start orchestrator` 会让人误以为新 harness 不可用，而错误实际上可能发生在旧 controller 或旧目录；底层错误留在 daemon 日志供诊断。

## TUI 启动命令的 Windows 边界

**首次启动与恢复代理必须使用同一套可执行命令语义。** OpenCode 的 TUI adapter 通过 `env KEY=VALUE … opencode …` 传入会话专属配置，OpenCode 2 还用它隔离数据目录。Windows 没有系统自带的 `env`；这些变量必须进入真实 agent 的子进程环境。

**PTY spawner 与 managed-agent supervisor 共用环境前缀解析。** 未加 supervisor 的命令由 PTY spawner 处理；加 supervisor 后，外层命令是 `ao agent-process supervise … -- env …`，只有 supervisor 看得到真正的环境前缀。Windows 的 supervisor 在执行子进程前应用这些赋值；Unix 保持原有执行语义。只解释 adapter 使用的连续 `NAME=VALUE` 前缀，缺少命令或使用其它 `env` 选项时保持原参数，交给原有错误路径处理。

**赋值只影响子进程，不修改 daemon、supervisor 或项目保存的配置。** 同名变量采用最后一个赋值；JSON、含空格或中文的路径、值中的等号及 shell 特殊字符都作为原始环境值和 argv 传递，不能拼接为 shell 脚本。stdout、stderr、stdin 和带 launch generation 的退出上报保持原来的所有权。

**PTY 创建成功只证明终端载体存在。** supervisor 内部的 agent 仍可能启动失败，并通过退出上报使会话显示为 `exited`。重复点击恢复会重建相同的 adapter 命令，因此无法修复确定性的命令兼容错误；该错误必须在实际执行边界解决。完整的 agent-ready 握手属于独立的启动协议扩展，本方案不以 UI 的 ready 记录替代这个保证。

## 不变量与易错点

**`is_terminated=true` 意味着 controller/provider 的退出已经确认。** 若先写终止状态、后异步 Stop，列表会显示结束而计费的 provider 仍工作。

**pending worktree release 是项目 Spawn 的前置条件。** 若新 session 复用旧 path 后才异步重试删除，清理可能删掉新 session 正在使用的文件。

**只把已接受的替换清理自动重试。** `session_cleanup_facts` 里的普通 pending/failed/dirty 仍按原有清理授权处理；不能为了替换成功而扩大到其他已终止 session。

**不要把 process-tree 强杀当作 Chat Stop 的后备。** Windows persistent host 已由 Chat driver 管理 provider tree；绕过 host 协议会失去退出确认、controller 收尾和事件结算。

**旧进程可能占用的恰好是新 Orchestrator 准备使用的规范 worktree。** 只把 filesystem 错误映射成 retry 或在设置页重开旧 session，都不会释放该目录；旧 owner 必须在释放步骤前退出。

## 被否决的方案

**保留替换通知，然后紧接着标记终止，被否决。** 通知会消耗一次真实模型调用，并且模型回复、工具调用或网关等待可能持续到 session 已标记终止之后。

**把 provider PID 写入 SQLite 后由通用 session manager 强杀，被否决。** persistent Chat host、provider 及其子进程的所有权在 driver/service；第二套 PID 所有权会与 host shutdown、daemon restart adoption 和跨平台进程树管理冲突。

**worktree 删除失败仍继续启动新 Orchestrator，被否决。** 两个 session 会争用同一个分支/目录；失败的 host 或异步清理也可能继续读写新 session 的工作区。

## 未实现的部分与扩展准入

项目级替换历史/诊断 UI 不在本方案范围内。以后增加时，应读取 durable cleanup facts 并区分旧 agent 停止失败与旧 worktree 释放失败；不得仅根据 session 的 `terminated` 展示“替换已完成”。

本机复现中的 Windows 错误只被结构化记录分类为 `filesystem`，未保存原始 OS 错误码。因此，本方案不假设具体是共享句柄、权限还是 Git 操作失败；所有这些失败都必须遵循同一条“旧 owner 已停止、清理已解决，才允许 Spawn”的边界。
