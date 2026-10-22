# Docker 云端部署验证记录

日期：2026-10-22。范围：用户授权部署空账本，域名与 HTTPS 按用户要求暂缓。没有执行 Git commit、push 或小程序发布。

## 环境与制品

- 腾讯云 Ubuntu，linux/amd64，Docker Engine 29.9.2、Compose v5.6.1。
- 使用官方腾讯云镜像加速源拉取 MySQL 8.4 系列、Go 1.27 和 Debian bookworm 基础镜像；Docker Hub 直连超时。
- Go 后端镜像 cost-api:local，架构 linux/amd64，运行用户 10001:10001。
- 上传包仅含 19 个必要的源码/配置/说明文件；不含本机 .env、二进制、Git 元数据、小程序代码或数据库数据。云端独立生成数据库密码和初始化密钥。
- 部署目录 /home/ubuntu/cost，生产项目 cost-cloud，测试项目 cost-verify-20261022。

## 本地检查

- 后端 go test ./... 通过，业务代码未改动。
- 初始化脚本 bash -n 通过，打包脚本 Python 语法检查通过。
- 白名单归档检查通过；Markdown 本地引用和 git diff --check 检查随交付执行。

## 首次空卷故障与修复

独立审查指出初始化脚本 set -eu 会被官方 MySQL entrypoint source，并使其读取未提供的 $1 时失败。隔离新卷运行实际重现 `$1: unbound variable`，业务账号未创建，数据库不健康。

将脚本改为 set -e，不向父入口传播 nounset。仅删除本次明确创建、带 cost-verify-20261022 标签的测试卷，重新新建空卷后，数据库初始化和认证查询健康检查通过。正式数据库在该修复验证后才创建。

这是真实空卷失败→修复→新空卷通过的集成验证；没有将同一半初始化卷重启作为修复证据。

## 隔离集成测试

- docker compose config --quiet：生产和测试配置均通过；必填配置缺失时失败。
- migrate 连续执行成功，schema_migrations 为 2，默认分类为 11。
- 新库初始 entries=0；后续只在测试库写入金额 1234 分、请求 ID deploy-persistence-check 的验证记录。
- 业务账号 INSERT/SELECT 成功，CREATE TABLE 被拒绝。
- API /health 返回 status=ok、auth_mode=wechat；未登录访问账单返回 401。
- API 容器访问 https://api.weixin.qq.com/ 返回 HTTP 404，证明 DNS/出站连接及 TLS 验证可用，不表示真实 wx.login 交换已通过。
- 强制重建测试 MySQL 和 API 后，验证记录仍存在且金额为 1234。
- mysqldump 备份成功；在无网络、无端口映射的独立临时 MySQL 容器中恢复成功，关键验证记录及金额一致。
- 临时恢复容器及匿名卷已清理；隔离测试服务已停止并移除。

## 正式空账本部署

- 正式卷首次创建、数据库初始化、两次显式迁移和 API 启动成功。
- 启动时确认 entries=0、budgets=0、users=0、family_members=0，categories=11，schema_migrations=2。
- api、mysql 均 healthy；restart 为 unless-stopped。
- API 非 root、只读根文件系统，端口只绑定宿主机 127.0.0.1:8080。
- MySQL 没有宿主机端口映射，根密码不进入 API 环境；HTTPS profile 未启动。
- 正式 /health 正常、未登录账单请求 401、微信域名 TLS 可达。
- 生成服务器初始备份 backups/cost-initial.sql.gz，并下载到本机独立备份位置；真实备份没有放入 Git 或公开交付包。
- Mac SSH 隧道映射 127.0.0.1:18080 到服务器 127.0.0.1:8080，Mac curl /health 正常，账单请求 401。

## 尚未完成和边界

- 首次云端管理员尚未绑定。Mac 上原有账单及登录状态没有迁移到云端。
- 没有修改用户当前开发者工具中的本地账本连接；用户可以手动切换到 SSH 隧道地址重新微信登录。
- 未在云端完成真实微信 code 交换、家人多人权限或手机真机验证；不要将 TLS 可达写成真实微信登录通过。
- 域名解析、腾讯云接入备案、HTTPS 和微信合法域名配置均暂缓。
- 已验证手动备份/恢复并做初始备份，尚未配置自动定时备份与保留策略。
- 应用启动健康检查只证明进程正常，不代表全部业务与发布审核完成。

## 独立审查

按 Superpowers requesting-code-review 进行了只读独立审查。Critical 初始化 nounset 问题已按真实失败证据修复；恢复手册已补充独立测试容器命令并实际演练。

审查暂未判断的范围：域名/HTTPS按用户要求不启用；Go main 已显式嵌入 time/tzdata，无需在运行镜像额外安装时区包；镜像可用性、空卷迁移及持久化由主执行会话的实际远端测试补齐。迁移采用显式运维顺序，不自动放入 API 启动流程。
