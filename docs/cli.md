# CLI 与结果契约

## 命令发现

`mybenchx --help` 和各子命令的 `--help` 面向人；`mybenchx describe --output json` 面向自动化。
`describe` 返回命令、网络/写入属性、示例、配置字段类型/默认值/环境变量、枚举、资源上限、退出码和延迟定义。
`workload list` 列出 point、range、datetime、unix 查询，以及 insert、update、delete 操作。

| 命令 | 行为 | 联网/写入 |
| --- | --- | --- |
| version / describe / workload list | 版本与能力发现 | 无 |
| config validate / config show | 校验/显示有效的非敏感配置 | 无 |
| plan | 显示配置、精确表清单、预填行数、写入性质和清理范围 | 无；不检查远端状态 |
| doctor | 连接、读取版本和表归属 | 联网、只读；tables_ready=false 不等于连接失败 |
| prepare | 创建 InnoDB 表并预填数据 | 要求 --execute；不创建数据库、不覆盖已有表 |
| run | 有界执行负载 | 有写 worker 时要求 --execute |
| cleanup | 删除配置精确指定的表 | 要求 --execute；先验证全部表的 schema 标记 |

旧 `random`、`seq`、`range` 命令不存在；不接受 XA 参数。不自动探测私人配置，不自动重试写入或清理。

## 配置

配置是 JSON 对象，字段见 `describe`。拒绝未知字段、重复 JSON key、尾随 JSON、错误类型和不合法参数。
优先级：命令行 > `MYBENCHX_` 环境变量 > 配置文件 > 默认值。字段名用 snake_case，命令行用 kebab-case。例如 `read_workers` 对应 `--read-workers` 和 `MYBENCHX_READ_WORKERS`。

`duration`、`timeout`、`report_interval` 使用 `60s`、`500ms` 等 Go 时长。`duration=0s` 必须配置正数 `max_operations`；`max_operations=0` 表示仅按时长限制。`run` 至少需要一个 worker。
默认只启用一个 read worker；设置 insert worker 不会自动清除 read worker。纯写负载请显式 `--read-workers 0`。

参数上限：1024 张表、总计 4096 个 worker、每条 INSERT 最多 10000 行、每批最多 10000 条语句，且 `batch * rows_per_insert <= 10000`，避免单 worker 预生成批次占用无界内存。每个 worker 固定占用一个数据库连接。

凭据只通过 `password_env` 或 `password_file` 引用，两者互斥。文件必须是 owner-only 普通文件（建议 0600），仅在连接时读取。SSH 私钥同样要求 owner-only。配置中的相对文件路径以当前工作目录为基准；私人用例建议用绝对路径。不支持明文 password 配置项或命令行密码。
不把密码放进示例、日志、结果和错误。`config show` 会展示主机、用户、数据库与凭据引用，不应当把它当匿名化报告公开发布。

`tls=required` 使用系统信任根验证 MySQL 服务端证书；`disabled` 为兼容已有环境的默认值。自定义 CA、SSH agent 和加密私钥解锁暂未提供。

## 工作负载与数据域

- `prepare` 使用 `base_time` 和行 ID 生成对应的 `created_at` / `unix_stamp`；连接会话使用 UTC，两个时间查询使用相同随机序列与时间域。
- 顺序插入使用 AUTO_INCREMENT；随机插入在启动时已有最大 ID 以上生成随机主键。重复键会明确失败，不静默重试。
- 点查、更新、删除在启动时各表实际 MIN/MAX ID 区间内取值；范围查询按方向选取有上限的结果。
- MIN/MAX 是启动快照，**不是“保证命中”的 ID 清单**。稀疏主键、随机插入和并发删除会产生零行结果，报告中的 `rows` 可以辨别。对比不同访问模式前应重新准备一致的数据集。
- `seed` 控制每个 worker 的数据生成；并发调度、提交顺序和实际服务端结果不保证完全重放。
- 普通查询只在成功耗尽并关闭结果集后计入成功。事务中语句执行成功还不等于提交成功。

## stdout 协议

`text` 显示进度和最终统计表；`json` 只输出一个最终对象；`ndjson` 每行一个独立 JSON 事件，顺序为 started、零到多个 progress、result。连接或参数错误可能直接返回 result。
stdout 不混入诊断日志或 ANSI 控制符；诊断走 stderr。`--help` 始终是人类帮助文本。

每个事件包含：

```json
{
  "schema_version": "1",
  "run_id": "UUID",
  "event": "result",
  "command": "run",
  "status": "completed",
  "time": "RFC3339 UTC",
  "data": {}
}
```

状态为 running、completed、failed、canceled。失败带 `error.code` / `error.message`；禁止从英文 message 推断成功与否。`run_id` 在一次调用的全部事件中一致。
run 的 data 包含有效 config、metrics 和 monitor；失败/取消尽可能保留部分结果。prepare/cleanup 的部分结果列出已经创建/删除的表，prepare.rows 只累计已成功写入的行。
表 DDL 和多个预填批次不是一个原子事务；失败后先检查部分结果，不自动再次 prepare 或扩大 cleanup 范围。

## 指标

| 字段 | 含义 |
| --- | --- |
| sql_attempted | 实际尝试执行的压测 SQL，不含发现查询和 BEGIN/COMMIT |
| sql_succeeded | 读操作完成，或写操作得到成功提交确认的语句数 |
| sql_failed | 失败的语句数 |
| sql_canceled | 因取消而中止的语句数 |
| sql_rolled_back | 在失败/取消事务中已经执行、随后回滚的语句数 |
| sql_commit_unknown | 写入或提交通信中断，无法确认是否提交的语句数 |
| rows | 成功操作实际返回/影响的行数；回滚和未知提交不计入 |
| transactions_committed | 已确认的写事务数，包含 autocommit；读操作不冒充写事务 |
| operations_per_second | 成功语句数 / 实际执行阶段秒数 |
| transactions_per_second | 成功写事务数 / 实际执行阶段秒数 |
| by_operation | read、insert、update、delete 各自计数及延迟 |
| interval | 仅进度事件提供：自上次进度以来的秒数、成功语句/秒、成功写事务/秒 |

`unit_latency` 是成功执行单元的客户端耗时。读单元包含完整结果读取，batch=1 写单元包含 autocommit，批量写单元包含 BEGIN、全部语句与 COMMIT。它不包含 SQL/随机数据预生成，不把批量事务延迟称为单条 SQL 延迟。
平均值为总耗时除以样本数，不再除以运行时长；分位数使用固定大小对数直方图，返回桶上界（限制于实际最大值），相对上界误差至多约 2.2%。无样本时返回 samples=0 和零值。

时长从发现数据范围和获取 worker 连接之后开始，到全部 worker 退出结束，不包含报告输出尾部耗时。
请求上限以 SQL 为单位，全 worker 共享预算，不超发；尾批可缩小，故实际事务可能少于 batch 条语句。失败/取消可能导致预算未用完。
停止原因是 max_operations、duration、canceled、initialization_error、error 或 output_error。时长届满是正常停止，但未知提交不会作为成功报告。

收到 Ctrl-C/SIGTERM 后取消查询、回滚未提交事务、等待 worker 退出并报告部分结果；MySQL 驱动可能关闭被取消查询占用的连接。**未知提交不得自动重试**，应先检查目标数据库。

## 退出码

| 退出码 | 含义 |
| --- | --- |
| 0 | 命令完成（运行时长/请求上限正常达到） |
| 2 | 参数、配置或写操作执行授权缺失 |
| 3 | 连接/认证失败 |
| 4 | 执行失败，可包含部分结果或未知提交 |
| 5 | 输出失败（例如管道关闭） |
| 130 | 用户取消 |

## SSH 监控

默认关闭。`monitor=ssh` 要求显式主机、用户、known_hosts 和私钥/密码环境变量引用。固定读取 Linux `/proc/stat`、`/proc/meminfo`、`/proc/diskstats`；不安装远端程序、不拼接用户输入到 shell。
CPU 为两次采样的利用率，内存为 MemAvailable。磁盘必须用 `ssh_disk` 选择一个设备，避免分区、逻辑卷和物理盘重复求和；未指定设备时磁盘指标缺省。磁盘速率来自计数差值，512-byte sector 转为 MiB/s。
第一次样本 warming_up，后续 available；采集/连接失败 unavailable，并给出原因。未采集的字段缺省，不填假零；监控失败不改变数据库负载结果。监控超时关闭当前 SSH 连接，不隐式无限重连。
