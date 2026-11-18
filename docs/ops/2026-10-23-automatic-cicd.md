# 自动 CI/CD

> 公开文档脱敏说明（2026-11-18）：个人路径、部署标识及真实账本数据已省略或改为占位示例。历史验证结论保留，不代表使用占位示例重新执行。示例路径使用前须替换；部署脚本的内置默认路径以源码为准，本次未调整脚本。

日期：2026-10-23。入口：`.github/workflows/ci-cd.yml`。本地文件完成不代表服务器接入或首轮自动部署成功，实际验证状态见末尾。

## 日常使用

Mac 修改代码并 push main 后，GitHub Actions 先检查 Go race/vet/build、独立 MySQL 8.4 集成测试、小程序测试、部署故障测试和 workflow 语法，再自动部署后端。PR 只检查，不读取生产 Secrets 或执行部署。小程序仍须在微信开发者工具上传、审核和发布。

在仓库 [Actions](https://github.com/LeeMrChang/cost/actions/workflows/ci-cd.yml) 查看结果。支持 main 的 Run workflow。部署串行，不因新提交取消已经开始的发布；排队中的旧提交会在执行前检查 main，过期则跳过。只有全部检查通过才进入部署。

## 一次性初始化

部署地址和账号使用自己的环境配置；下文 `192.0.2.10` 为文档专用示例 IP，`deploy-user` 为占位账号，不能直接用于连接。路径示例为 `/path/to/server-user/cost`；初始化脚本的固定路径须先与实际环境核对。Mac 需要 `gh`、Python 3、SSH；账号必须能管理本仓库 Secrets。请在 Mac 终端执行，不把密码粘贴到源码：

```sh
cd /path/to/cost
gh auth login --hostname github.com --git-protocol ssh --scopes workflow
bash scripts/setup-cicd.sh 192.0.2.10 deploy-user
```

脚本读取 Mac 已核验的 known_hosts，检查现有生产 `.env`、Docker Compose、免交互 Docker 权限，然后生成专用 Ed25519 密钥，把公钥加入服务器 authorized_keys、私钥写入仓库 Secrets；临时本地私钥在退出时删除，不复用 Mac 个人私钥。SSH 需要密码时仅在 SSH 提示中输入。服务器账号需要 Docker 或免密 `sudo -n docker`，Ubuntu 常见初始账号已具备；脚本不会自动修改 sudoers、SSH 密码或数据库配置。

脚本设置以下 Secrets：

| Secret | 用途 |
|---|---|
| COST_DEPLOY_HOST | SSH 地址 |
| COST_DEPLOY_USER | SSH 用户 |
| COST_DEPLOY_PORT | SSH 端口，默认 22 |
| COST_DEPLOY_SSH_KEY | 独立部署私钥 |
| COST_DEPLOY_KNOWN_HOSTS | 已核验服务器主机公钥 |

如果 host key 尚未记录，先通过服务器控制台核对指纹并建立一次可信 SSH 连接，再运行初始化。不能用关闭 StrictHostKeyChecking 替代。初始化失败可重跑；脚本输出的 key comment 用于识别 authorized_keys 中本次添加的条目。轮换成功后删除旧 comment 对应公钥。该部署账号的 Docker 权限等价主机管理权限，只向可信维护者开放 main 和 Secrets 管理。

工作流进入 main 且 Secrets 设置成功后，首次可手动触发，无需创建空提交：

```sh
gh workflow run ci-cd.yml --repo LeeMrChang/cost --ref main
gh run list --repo LeeMrChang/cost --workflow ci-cd.yml --limit 5
```

云防火墙需允许 GitHub 托管 runner 的 SSH 连接；若当前 22 端口仅允许固定个人 IP，需先设计适当的网络接入策略。运行时会严格验证服务器 host key。

## 服务器执行过程

1. 上传白名单源码到 `incoming/SHA-run-attempt.tar.gz`，使用 `flock` 排他锁；拒绝路径穿越、链接、生产 `.env` 或其他非白名单文件。
2. 解包到不可变 `releases/SHA-run-attempt/`；沿用生产 `deploy/.env`、Compose 项目名 `cost-cloud` 和现有命名数据卷。
3. 保存并标记原 API 镜像，先构建新镜像；构建期间原 API 继续运行。
4. 停止 API，备份到权限受限的 `backups/ci-*.sql`；用相同 MySQL 镜像创建无网络临时容器，恢复并比较全库有序数据 dump 的 SHA-256，完成后仅移除临时容器及其匿名卷。
5. 显式执行新镜像迁移，再 `up --no-deps --wait api`；不重建生产 MySQL，不调整微信身份或合并其他数据库。
6. Compose health 和 `/health` 通过后，更新 `current` 符号链接，保存 `current/image.env` 和 `current/SUCCESS`。健康检查证明服务启动，不能代替真实微信和多人真机验收。

部署会短暂停服，备份及恢复验证期间同样停服；当前家庭账本数据量适用此方案。不要在部署期间通过其他工具写库。备份、历史源码和镜像先保留，不自动 prune；需要定期检查磁盘并建立异地备份及保留策略。已有镜像服务配置可保留 Caddy；CI 不重启 HTTPS 服务或更改域名。

## 失败与手动操作

- CI 失败：不连接生产，修复后 push。
- 构建失败：旧 API 继续运行。
- 备份或恢复验证失败：不迁移，尝试启动原容器；检查实际 health。
- 迁移开始后任意失败：API 保持停止，保留现场与备份，需检查迁移是否部分完成后选择重试或兼容版本；不自动用旧备份覆盖数据，不假设旧镜像支持新结构。
- 首轮失败可从 Actions Re-run jobs 重试，新 attempt 使用新 release ID；相同成功 ID 不重复执行。

首次自动部署成功后，手动运维应使用 current 指向的 Compose 文件和记录的镜像标签，避免旧目录 `.env` 中 `COST_IMAGE_TAG=local` 启动旧版本。服务器上：

```sh
cd /path/to/server-user/cost
set -a
. current/image.env
set +a
sudo --preserve-env=COST_IMAGE_TAG docker compose --project-name cost-cloud \
  --env-file /path/to/server-user/cost/deploy/.env \
  -f /path/to/server-user/cost/current/deploy/compose.cloud.yaml ps
```

若 sudo 策略不允许 preserve-env，可改为 `sudo env COST_IMAGE_TAG="$COST_IMAGE_TAG" docker compose ...`。`logs --tail 80 api` 可查看服务日志，勿输出完整环境配置。只有确认数据库兼容才可手动选择历史镜像；结构回退和备份恢复需单独安排。MySQL 镜像、初始化账号和 Caddy 配置的升级另按运维流程处理。

## 本次验证状态

- 部署状态机测试先因缺少脚本失败，实现后 11 项通过；覆盖构建、备份、恢复不一致、迁移、健康失败及归档攻击、`.env` 保留和重复成功发布。按真实 `bash -s` 输入方式重现并修复 Compose 误读脚本 stdin；另重现并修复 sudo 丢失镜像标签。两项均看到失败再修复通过。
- 本地 Node 三组测试、页面/语法检查、Go race/vet/build、Bash 语法、git diff --check 通过。Go 本地 MySQL 集成测试因连接受限未执行，交由 CI 独立 service 实跑。
- actionlint v1.7.12 本地通过（下载包 SHA-256 与官方 release digest 一致）；本地未安装 shellcheck，actionlint 本地显式关闭其集成，GitHub runner 上按默认检查。
- 按 requesting-code-review 完成独立只读审查，发现的 stdin 问题已修复并经复审；独立重跑 11 项测试及 Bash 语法通过，无剩余 Important 发现。
- 当前会话 SSH 连接被环境限制拒绝；gh auth status 检查失败。Git SSH push 无法解析 github.com。GitHub 连接器已创建 feature/automatic-cicd 远端分支，但批量写入在创建 tree 时返回 404，代码未发布成功；连接器也不提供 Secrets 管理。因此尚未配置远端密钥或 Secrets，尚未执行首轮 CI/自动部署。

当前实现保存在本地 feature/automatic-cicd 分支。完成上方初始化后，可将该分支快进推送到 main 以启用工作流（远端有新提交时 Git 会拒绝，应先整合，禁止 force push）：

```sh
git push origin feature/automatic-cicd:main
```

参考：[GitHub workflow 语法](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)、[部署并发控制](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/control-deployments)。

## 首轮云端运行：默认 socket 连接失败

2026-10-23 手动运行 [39909557544](https://github.com/LeeMrChang/cost/actions/runs/39909557544)：CI、SSH、镜像构建成功；停止 API 后，mysqldump 报错 2002，无法连接 `/var/run/mysqld/mysqld.sock`。失败发生在备份阶段，尚未迁移；失败处理已请求重启原 API，实际健康状态仍需服务器检查。

原因：部署脚本的客户端省略 host/protocol，默认查找 Unix socket；既有 MySQL healthcheck 使用 TCP，健康检查通过并不能保证客户端默认 socket 可用。修复将生产和临时恢复容器的 mysql/mysqldump 统一指定 `--protocol=tcp -h127.0.0.1`。临时容器的 TCP 就绪检查也避免误连初始化期间的临时 socket 服务。停止 API 前增加 root TCP 查询预检，连接或认证失败即退出，不进入停服阶段。

新增“默认 socket 不存在仍完成备份与恢复”和“预检失败不停止 API”测试，修改前两项均失败；修复后总计 13 项部署测试通过。Bash 语法、差异检查通过。测试使用隔离 Docker 替身，本机没有 Docker，真实生产验证以修复合并后的新 Actions 运行为准；重跑旧提交不会加载本修复。

连接行为参考：[MySQL 8.4 连接协议说明](https://dev.mysql.com/doc/refman/8.4/en/transport-protocols.html)。


## 2026-11-11：图标迁移与发布资源

正常发布流程已经包含备份、隔离恢复验证及显式migrate，010图标迁移随对应新版后端镜像执行。只重启API不迁移数据库。新增Go embed数据文件必须同时进入package-cloud.py与deploy-cloud.sh的精确白名单；当前category_icon_catalog.json已加入，打包与远端解包测试及发布包源码构建通过。该工作区变更尚未提交或发布，不代表云端当前已执行010。
