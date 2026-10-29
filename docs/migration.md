# 迁移与私人用例

本次为 CLI 和数据契约变更，不保留旧命令别名，不保留 XA。旧版本输出存在统计公式和事务缺陷，新旧数值不能直接比较为数据库性能变化。

| 旧入口/参数 | 新入口/参数 |
| --- | --- |
| random / seq | run --access random / sequential |
| range | run --workload range |
| query-type common/time_stamp/unix_stamp | workload point/datetime/unix |
| write/read/update/delete-threads | insert/read/update/delete-workers |
| max-time（整数秒） | duration（例如 60s） |
| max-request | max-operations（精确限制 SQL 尝试数） |
| batch-per-commit | batch（每个写事务的语句数） |
| oltp-tables-count / oltp-table-size | tables / rows |
| mysql-* 连接参数 | host / port / user / database |
| 明文 mysql-password | password-env 或 password-file 引用 |
| is-rds | monitor off；需要主机采集时 monitor ssh |
| mysql-enable-xa | 已删除，无替代入口 |

新表使用可配置前缀，默认 `mybenchx_`，并带 `mybenchx:v1` 表注释标记。旧无标记表不会被 run/cleanup 自动接管。请为新基线使用新表；不要为了绕过保护给旧表补标记。当前 schema 使用 InnoDB，不继续暴露旧 TokuDB 引擎选项。

私人连接与环境专用用例必须放 `.local/`；它由根 `.gitignore` 完整忽略。不要 `git add -f .local`。私人密码文件权限 0600、目录权限 0700。
已经存在于旧提交中的秘密不会因 .gitignore 或新版本删除而从 Git 历史消失；仍有效凭据需由所有者轮换，历史重写另行处理。

重构现场的 `.local/legacy/` 保存原始源码及未提交差异；`.local/cases/` 保存转换后的配置，`.local/secrets/` 保存密码，`.local/cases.json` 保存原函数映射，`.local/run-case.py` 提供默认离线预览。它们是各机器的私人资料，不会随 clone 分发。
`examples/` 只包含虚构连接示例与环境变量引用；通用测试不依赖任何私人连接。
