# 自动 CI/CD 实施计划

**Goal:** main push 自动检查并部署现有 Cost 云端后端。
**Architecture:** GitHub Actions + 白名单源码归档 + SSH + 服务器 Compose；生产秘密只在服务器。
**Tech Stack:** Bash、Python unittest、Go、Node、MySQL 8.4、Docker Compose。
**Design:** [设计](../specs/2026-10-23-automatic-cicd-design.md)。在当前会话顺序执行；独立代理仅用于最终代码审查。

## Task 1：部署故障验证

- [x] 新增 scripts/tests/test_deploy_cloud.py，用隔离临时目录及替身 docker/curl 检查执行顺序和失败分支。
- [x] 运行 python3 -m unittest discover -s scripts/tests -v，确认部署脚本缺失导致红灯。
- [x] 实现 scripts/deploy-cloud.sh：锁、归档校验、不可变 release、构建、停止 API、备份恢复、迁移、健康验证；重复运行已成功 release 跳过。
- [x] 验证构建失败不停止旧 API、备份/恢复失败恢复原 API、迁移失败不启动旧版、健康失败不发布 current、无效归档不操作容器；补齐 bash -s 输入和 sudo 环境变量回归，总计 11 项通过。

## Task 2：工作流与初始化

- [x] 新增 .github/workflows/ci-cd.yml：PR/main 检查、MySQL service、Node/Python/Go、actionlint；仅 main 成功后部署，不取消运行中的部署。
- [x] 新增 scripts/setup-cicd.sh：检查 gh、SSH host key、现有部署、sudo Docker；生成独立密钥并安装，配置五个仓库 Secrets。
- [x] bash -n、YAML/actionlint、本地业务测试；本机不能完成的 Docker/SSH 验证明确记录。

## Task 3：交付

- [x] 新增 docs/ops/2026-10-23-automatic-cicd.md，更新 README 和 docs/README：初始化、触发、日志、失败处理、手动运维与回退限制。
- [x] 按 requesting-code-review 独立只读审查并处理问题；复审无剩余 Important，独立重跑 11 项测试与 Bash 语法通过。
- [x] 本地提交并尝试推送：Git SSH DNS 失败；连接器创建分支成功，但写 tree 返回 404，未发布代码。提供初始化和 main 快进推送命令。
- [ ] 真实接入：安装服务器部署公钥、设置五个 Secrets、推送 main、确认 GitHub MySQL CI 和首轮云部署成功。SSH 连接被沙箱禁止，gh 登录检查失败；需要在可联网且有登录权限的 Mac 终端执行文档命令。
