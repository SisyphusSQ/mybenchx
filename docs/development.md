# 开发与验收

## 工具链与结构

使用 Go 1.27，推荐工具链固定为 1.27.3。模块依赖仅由 go.mod / go.sum 管理；不提交 GOPATH 缓存、二进制和私人资料。

- cmd/mybenchx：信号与进程退出。
- internal/cli、config、report：输入输出契约。
- internal/mysql、workload、bench：表归属、SQL、并发事务和统计。
- internal/monitor：可选且校验身份的 SSH 采集。

```bash
make build
make test
make race
make vet
make cross
```

build 只构建，不捆绑测试；cross 生成 Linux/macOS 的 amd64/arm64 二进制到 dist/。VERSION 可由构建参数注入。默认测试不访问外部服务，覆盖配置、CLI 协议、事务失败计数、请求预算、统计和监控解析；SSH 协议测试只启动临时 loopback 测试服务。

## 显式远端验收

不自动启动本机 MySQL，不自动选择 `.local/` 中的任何连接。先确认你获准使用该目标，再设置：

```bash
MYBENCHX_INTEGRATION_CONFIG=/absolute/path/to/authorized-private-config.json make integration
```

该测试仅取指定配置的连接与凭据引用，使用有限 worker、每表 30 行、随机独立 `mbx_it_` 前缀，验证：

- 建表/预填、已有表拒绝覆盖、各查询与两种访问模式。
- autocommit/批量插入的提交行数与精确操作上限。
- 混合负载、时长停止、初始化/运行中取消、查询超时。
- NDJSON 开始/进度/结束协议。
- 清理拒绝无归属标记的表。

测试结束清理本轮拥有的表；清理失败会报告，不扩大范围。不要把此入口用于未经授权的数据库。所有私人配置和运行日志留在 `.local/`，不上传原始配置或结果到公开 Issue。

CI 仅执行离线测试、race、vet、交叉构建、依赖一致性和私人文件跟踪检查，不注入私人数据库凭据。远端 CI 成功需以实际运行结果为准。

## 审查重点

事务提交/回滚结果、连接与 goroutine 退出、统计分母与单位、stdout 协议、清理表归属、私人文件忽略、敏感错误消息。
真实远端 SSH 主机监控需在获得对应主机授权后单独验收；解析测试不等于真实主机验收。
