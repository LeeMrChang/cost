# Redis 容器与记账返回首页诊断

2026-11-04。用户要求在现有腾讯云服务器新建 Redis Docker 容器，先供 Cost 使用，并评估首页缓存。用户补充：慢主要发生在新增账单后返回首页。

## 范围与结论

本轮先提供独立 Redis 部署工具和只读请求耗时诊断，不未经测量直接缓存资金数据。Redis 可用于重复读取，但新增账单后统计、账户余额、账单缓存均需失效；“返回慢”不能仅靠命中上次旧数据解决。

源码确认：首页每次 onShow 清空列表并刷新 6 个接口，Promise.all 等待 5 个业务接口完成后统一展示；头像资料独立刷新。每个微信认证请求都会查会话，再经 LedgerForUser 开事务锁用户行并查账本。统计读取整个期间的账单；账户余额查询汇总账单和转账。以上是潜在耗时点，未取得线上延迟证据，不能认定某个 SQL 已经慢。

## Redis 部署（实施者选择）

- 独立 Compose 项目 `cost-cache`，服务 `redis`，容器名由 Compose 生成 `cost-cache-redis-1`。不修改当前 Cost/Resume 服务，不重启 API/MySQL。
- 固定官方镜像 `redis:8.4.8-alpine`；首次安装允许 `COST_REDIS_IMAGE` 指向服务器可用的镜像源。标签以当日官方镜像清单核验。
- 加入已经存在的 `cost-cloud_default` 网络，别名 `cost-redis`；允许首次设置 `COST_REDIS_NETWORK` 适配实际网络。无主机端口映射，不开放公网 6379。
- 非 root 用户 `redis`，只读根文件系统、临时配置、密码认证；首次在服务器生成 64 位十六进制随机密码，`.env` 权限 600，不写入 Git、不输出到终端。
- 数据内存上限 128 MiB、容器上限 256 MiB、CPU 上限 0.5 核，`allkeys-lru`；关闭 RDB/AOF，仅作可重建缓存。丢失缓存不应丢失账单。
- 安装目录默认 `/home/ubuntu/cost/redis`。受管标记、排他锁及配置一致性检查保护重跑；不接管已有未知容器、目录或覆盖已有密码。
- 安装工具只启动/验证该 Redis 服务，校验认证 PING 和没有端口映射。后端尚无 Redis 客户端，本轮不声称已接入缓存。

## 诊断与后续缓存决策

只读诊断脚本从 `docker logs` 的 request 日志提取方法、已知接口路径和 duration，输出 count/p50/p95/max；不输出日志原文、查询词、用户编号、会话或账单内容。同时按运维文档采集容器 CPU/内存，复现新增账单后返回首页。

若服务端仅数毫秒而手机耗时明显，优先看网络、6 次认证/请求与清空后整页等待；若 stats/accounts 耗时突出，再考虑按用户/账本/月份隔离的短缓存。建议候选：分类、账本资料 5 分钟；月统计 30 秒；账户余额和账单首屏最多 10–30 秒。具体 TTL 只是待测方案，不是已执行配置。

后续实现必须满足：认证仍每次执行且不缓存登录会话；成功写入账单、恢复/删除、转账、预算、分类、账户、账本资料后失效相关缓存；处理并发旧读回填；Redis 故障有短超时并回退 MySQL；按北京时间日期分隔跨日预算计算；提供命中/未命中耗时指标。优先评估首页聚合接口减少认证轮次，避免把更长的旧数据展示当作提速。

## 验证与边界

行为测试运行安装脚本，验证凭证保留、未知目录/容器拒绝、网络缺失、启动失败及无公网端口；诊断使用固定日志验证分位数与敏感内容过滤。可用 Docker 时另验证真实容器认证、内存和健康。

当前直接 SSH 报 Operation not permitted，桌面工具报 native pipe startup failed；本机未安装 Docker/Redis，不能操作服务器或声称容器已启动。准备可上传工具包后，仍需要可用的服务器执行通道才能完成部署和实测。截图中的 backup 重启 126 另需读取实际日志，不能据截图判断与首页延迟有因果关系。

参考：[官方镜像标签](https://github.com/docker-library/official-images/blob/master/library/redis)、[Redis 内存淘汰](https://redis.io/docs/latest/reference/eviction/)、[Redis 安全](https://redis.io/docs/latest/operate/oss_and_stack/management/security/)、[Compose 网络](https://docs.docker.com/reference/compose-file/networks/)。
