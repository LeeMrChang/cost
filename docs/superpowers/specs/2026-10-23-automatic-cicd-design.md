# 自动 CI/CD 设计

> 公开文档脱敏说明（2026-11-18）：个人路径、部署标识及真实账本数据已省略或改为占位示例。历史验证结论保留，不代表使用占位示例重新执行。示例路径使用前须替换；部署脚本的内置默认路径以源码为准，本次未调整脚本。

日期：2026-10-23。依据：用户在了解 GitHub Actions 自动测试及云端部署流程后要求配置“自动 CI/CD”。

## 范围与选择

采用 GitHub 托管 runner 测试、SSH 上传白名单源码、服务器 Docker Compose 构建。相比在生产服务器安装自托管 runner，PR 代码不接触生产主机；相比引入镜像仓库，复用现有服务器构建和国内镜像配置。PR 执行检查；main push 检查成功后自动部署；支持 main 的手动重跑。不自动发布微信小程序。

## CI

Go race、vet、build，使用独立 MySQL 8.4 service 运行全部数据库集成测试；Node 执行小程序现有测试及文件检查；Python 执行部署故障测试；actionlint 检查 workflow。PR 不注入生产密钥。源码包复用 scripts/package-cloud.py 的白名单。

## 部署

固定生产目录以脚本配置为准（公开路径示例：/path/to/server-user/cost）、Compose 项目 cost-cloud。每次使用 SHA-run_id-run_attempt 标识不可变 release 和镜像。服务器既有 deploy/.env 保留，数据库密码及微信秘密不上传 GitHub。

部署串行且不因新 push 取消进行中的部署，服务器另用 flock 互斥。先检查已有 MySQL/API 和生产配置，再构建镜像；停止 API 后备份数据库，恢复到无网络临时 MySQL，核对数据摘要；验证成功后显式 migrate，最后只重建 API，等待 health 通过。不重建生产 MySQL、删除卷或自动导入账单。

构建失败不影响旧 API；备份/恢复失败重新启动原容器；一旦开始迁移，失败保持 API 停止并保留备份供人工处理，不能猜测旧镜像和新结构兼容。成功保存 current 指针及镜像标签用于运维。

## 权限与接入

仓库 Actions 最小 contents:read；仅 main 部署步骤读取专用 SSH 密钥和已核验主机公钥。初始化脚本生成独立部署密钥，添加禁用转发/PTY 的 authorized_keys 条目，再用 gh secret set 上传；不复用 Mac 私钥。SSH 账号需要对部署目录有权限并可免密 sudo Docker；该权限等价服务器管理员能力，密钥仅供 main 部署。

本会话 SSH 受沙箱禁止，gh 登录检查失败；远端安装、Secrets 与 Actions 实跑必须区分于本地文件完成。配置以本地测试、独立代码审查、GitHub 真实运行和云端健康状态作为分层验收，不伪造上线证据。
