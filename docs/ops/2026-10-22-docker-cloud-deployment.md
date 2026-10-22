# Docker 云端部署与运维

日期：2026-10-22。适用：Ubuntu、linux/amd64、空账本。首次数据库不导入 Mac 的用户、账单、会话或预算。

## 首次上传的具体操作（新环境）

当前服务器已经由本次部署完成，以下用于后续复现，不要重复覆盖正在使用的配置。

Mac 终端：

```sh
cd /Users/silent/Documents/work/private/cost
archive_file="/tmp/cost-cloud-$(date +%Y%m%d%H%M%S).tar.gz"
python3 scripts/package-cloud.py --output "$archive_file"
scp "$archive_file" ubuntu@服务器公网IP:/home/ubuntu/cost-cloud-source.tar.gz
```

服务器终端：

```sh
mkdir -p /home/ubuntu/cost
chmod 700 /home/ubuntu/cost
tar -tzf /home/ubuntu/cost-cloud-source.tar.gz
tar -xzf /home/ubuntu/cost-cloud-source.tar.gz -C /home/ubuntu/cost
cd /home/ubuntu/cost/deploy
```

先确认 `.env` 不存在，才从示例创建；存在时保留它并检查是否已有部署：

```sh
if [ ! -e .env ]; then
  cp .env.example .env
  chmod 600 .env
fi
openssl rand -hex 32
```

分别生成三个独立数据库密码和一个初始化密钥，填写对应键。用 `nano .env` 编辑，填写 AppID/AppSecret；仅在服务器终端查看这些值，不发送到聊天。服务器访问 Docker Hub 超时时，可使用模板内的官方腾讯云镜像源和 Go 模块代理选项。确认配置后，再执行下方迁移/启动命令。

## 配置和顺序

部署目录 /home/ubuntu/cost；命令默认在其 deploy/ 中执行。私有 .env 仅放服务器，chmod 600。三个数据库密码分别使用 openssl rand -hex 32 生成，必须为 64 位十六进制，不复用。填写小程序 AppID/AppSecret 及初始化密钥；后端镜像构建不读取 .env。

Mac 打包：`python3 scripts/package-cloud.py --output /tmp/cost-cloud.tar.gz`。只包含后端构建源码、云端配置和本说明，不包含小程序、本地秘密或账单。通过 scp 上传，服务器先检查归档再解包。已有部署更新前先备份目录与数据库，不覆盖既有 .env。

从 deploy/ 执行：

```sh
sudo docker compose --env-file .env -f compose.cloud.yaml config --quiet
sudo docker compose --env-file .env -f compose.cloud.yaml build api
sudo docker compose --env-file .env -f compose.cloud.yaml up -d --wait mysql
sudo docker compose --env-file .env -f compose.cloud.yaml --profile tools run --rm migrate
sudo docker compose --env-file .env -f compose.cloud.yaml up -d --wait api
curl -fsS http://127.0.0.1:8080/health
sudo docker compose --env-file .env -f compose.cloud.yaml ps
```

迁移成功后才启动 API；以后每次升级也先备份、迁移、再重建 API。不要把迁移塞入每次服务启动。mysql 初始化脚本只在新卷上运行；已有卷时不能靠修改 .env 自动修改数据库密码，须单独设计轮换。

## 网络、域名和真实联调

API 宿主机只绑定 127.0.0.1:8080，MySQL 无宿主机端口。MySQL 业务账号只有 DML，migrate 使用本库专用 DDL 账号。后端可出站访问微信 HTTPS。

Mac 建立隧道：

```sh
ssh -N -L 18080:127.0.0.1:8080 ubuntu@服务器公网IP
```

保持隧道终端运行。微信开发者工具登录页设置 http://127.0.0.1:18080，保存调试地址会清除旧会话，再重新微信登录。首次云端管理员使用云端初始化密钥；成功后移除该配置并重建 API。该隧道仅对 Mac 模拟器有效，手机的 127.0.0.1 不指向 Mac。

默认接口域名 api.xiachen.xyz。域名已在其他接入商备案时，先按腾讯云要求办理接入备案。域名控制台添加 api 的 A 记录指向实际服务器公网 IP；避免误删根域名、邮件或其他服务记录。确认没有指向错误主机的 AAAA。

接入条件满足、DNS 生效、云防火墙允许 TCP 80/443 后，才启动 HTTPS：

```sh
sudo docker compose --env-file .env -f compose.cloud.yaml --profile https up -d --wait https
```

Caddy 自动申请/续期证书并转发到 api:8080，数据保存在独立卷。检查 https://api.xiachen.xyz/health；若证书申请失败，先检查 DNS、接入备案及入站端口，不反复重建尝试。微信后台配置 request 合法域名，小程序发布配置切换为 HTTPS 域名后再真机验证。本阶段部署不自动发布小程序。

## 日志、重启和停止

```sh
sudo docker compose --env-file .env -f compose.cloud.yaml logs --tail 80 api mysql
sudo docker compose --env-file .env -f compose.cloud.yaml restart api
sudo docker compose --env-file .env -f compose.cloud.yaml stop
```

容器 restart:unless-stopped；显式 stop 后不会自行恢复业务，需执行 up -d。日志每文件上限 10MB，保留 3 份。不要输出展开后的真实 compose config 或 inspect 环境变量到聊天。

数据卷不是备份，不执行 down -v。不要覆盖包含真实数据的卷来重试首次部署。

## 数据库备份和恢复验证

服务器使用单独文件保存 SQL，限制权限。下例在 deploy/ 执行，容器内读取现有密码，不在命令行打印密码：

```sh
umask 077
mkdir -p ../backups
backup_file="../backups/cost-$(date -u +%Y%m%dT%H%M%SZ).sql"
sudo docker compose --env-file .env -f compose.cloud.yaml exec -T mysql sh -c \
 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --no-tablespaces --set-gtid-purged=OFF cost' > "$backup_file"
```

确认命令退出码为 0、文件非空再压缩，并定期将备份下载到独立位置。备份包含用户/会话/账单，禁止提交 Git 或分享内容；建议在受控备份系统中加密并执行保留策略。

恢复必须先在独立临时 MySQL 容器/数据库验证。只导入备份业务表不包含 mysql 系统账号，恢复后重新准备运行账号和迁移账号。比较表数、记录数和关键数据，再做应用只读验证。生产恢复会覆盖数据，需用户单独确认，不能直接把测试命令指向生产库。

### 在独立临时容器演练恢复（只用于测试）

下例使用全新、无公网端口的临时容器，不指向正式数据库。需要已有可用备份文件 `$backup_file`。在 deploy/ 执行；如果同名测试容器已存在，先停下检查，不删除不明资源。

```sh
umask 077
restore_env=$(mktemp)
printf 'MYSQL_ROOT_PASSWORD=%s\n' "$(openssl rand -hex 32)" > "$restore_env"
mysql_image=$(sudo docker compose --env-file .env -f compose.cloud.yaml images -q mysql)
sudo docker run -d --name cost-restore-check --network none --env-file "$restore_env" "$mysql_image"
```

等待数据库完成初始化（最多约两分钟）：

```sh
restore_ready=0
for attempt in $(seq 1 60); do
  if sudo docker exec cost-restore-check sh -c \
    'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --protocol=tcp -h127.0.0.1 -uroot -e "SELECT 1"' >/dev/null 2>&1; then
    restore_ready=1
    break
  fi
  sleep 2
done
```

确认 `restore_ready` 为 1 才执行后续导入，否则查看测试容器日志排查：

```sh
test "$restore_ready" = 1 && echo '测试库可连接'
sudo docker exec cost-restore-check sh -c \
 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -e "CREATE DATABASE cost CHARACTER SET utf8mb4"'
sudo docker exec -i cost-restore-check sh -c \
 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot cost' < "$backup_file"
sudo docker exec cost-restore-check sh -c \
 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -Dcost -e "SHOW TABLES; SELECT COUNT(*) AS entries FROM entries; SELECT COUNT(*) AS categories FROM categories;"'
```

导入必须退出码为 0，表、行数与关键业务字段必须和备份来源一致；之后才可认为恢复演练通过。这个临时实例不保存原 MySQL 系统账号，不能直接用作生产替代。

确认完成后，只清理本例亲自创建的测试容器及匿名卷：

```sh
sudo docker rm -fv cost-restore-check
rm -f "$restore_env"
```

### 本次初始化脚本兼容性修复

官方 MySQL 入口会 source 非可执行的初始化 .sh，因此初始化脚本不得设置 `set -u` 污染父入口环境。隔离测试曾实际触发 `$1: unbound variable`；修复后必须重建明确的测试卷重新验证。正式卷失败时不直接删除，先备份并调查。
