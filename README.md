# mybenchx

基于 Go 的 MySQL 压测工具，提供可验证的工作负载、明确的事务/统计语义，以及面向 AI 和脚本的 CLI 协议。

支持建表预填、顺序/随机访问、点查/范围查询、datetime/unix 时间索引对比、增删改及可选 SSH Linux 主机监控。默认只读负载，写操作要求显式 `--execute`。旧命令和 XA 已移除。

## 构建

使用 Go 1.27（推荐工具链 1.27.3）：

```bash
make build
./bin/mybenchx --help
./bin/mybenchx describe --output json
```

产物为 `bin/mybenchx`。`make cross` 构建 Linux/macOS amd64/arm64 版本；`make build VERSION=...` 注入版本。

## 一次完整操作

先复制 `examples/read.json` 到被忽略的 `.local/`，填写你获准使用的目标。数据库需预先存在。示例引用 `MYBENCHX_DB_PASSWORD`，通过环境或权限受限文件提供秘密，不在命令行中传密码。

```bash
# 以下两步离线，不访问数据库
./bin/mybenchx config validate --config .local/bench.json --output json
./bin/mybenchx plan --config .local/bench.json --output json

# 显式访问目标；doctor 不写入
./bin/mybenchx doctor --config .local/bench.json --output json
./bin/mybenchx prepare --config .local/bench.json --execute --output json

# 只读压测；NDJSON 会持续输出进度并在结束时给出汇总
./bin/mybenchx run --config .local/bench.json --duration 60s --output ndjson

# 纯插入示例：显式清除默认 read worker，并授权写入
./bin/mybenchx run --config .local/bench.json --read-workers 0 --insert-workers 2 --batch 10 --max-operations 1000 --duration 0s --execute --output json

# 清理仅作用于配置指定且归属当前 schema 的表
./bin/mybenchx cleanup --config .local/bench.json --execute --output json
```

`--output text` 面向终端，`json` 只输出最终对象，`ndjson` 输出事件流。错误也遵循所选协议，退出码区分配置、连接、执行、输出失败和取消。检查 status/退出码及部分结果，不自动重试失败的写操作。

## 指标与边界

成功 SQL 次数、事务数、实际返回/影响行数、失败、取消、回滚和未知提交分别记录。延迟以完整执行单元为口径，批量事务包含提交；提供平均/最小/最大和 P50/P95/P99。日志不会混入机器协议 stdout。

查询使用实际表的启动时取值范围；稀疏数据及并发删除可能返回零行。随机种子控制生成数据，不保证并发调度重放。比较不同场景前应准备一致数据集，结合 rows 评估命中情况。

新表带 `mybenchx:v1` 标记。不会自动复用/清理旧版本无标记表。原有私人配置、用例和现场备份放 `.local/`，禁止提交；历史凭据不会因新版本删除而从 Git 历史消失。

## 开发

```bash
make test race vet
# 仅在目标已获授权后，显式启用集成验收；不启动本机数据库
MYBENCHX_INTEGRATION_CONFIG=/absolute/path/to/private-config.json make integration
```

[文档索引](docs/README.md) · [CLI 契约](docs/cli.md) · [迁移说明](docs/migration.md) · [开发与验收](docs/development.md) · [Issue #5](https://github.com/SisyphusSQ/mybenchx/issues/5)

本项目源自 [xelabs/benchyou](https://github.com/xelabs/benchyou)，保留原仓库 LICENSE 与来源归属。每操作统计的原始思路受 Mark Callaghan 的 Small Datum 启发。
