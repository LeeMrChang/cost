# CI 数据库字段数量断言修复

2026-11-03，PR #13。用户反馈 Tests and checks 失败。通过浏览器读取 [失败作业](https://github.com/leech-io/cost/actions/runs/41618623709/job/125858293246) 的真实日志，确认三个失败均为 `columns=61 uncommented=0 err=<nil>`：TestMySQLPrefixedFresh、TestMySQLPrefixPreservesLegacy、TestMySQLPrefixResumeAfterRename。

原因：007 新增 nickname 后字段数为 61，旧前缀测试仍写死 006 的 60 列。修复将总数约束改为不少于原结构 60 列，继续检查全部字段均有注释、9 张表前缀/注释及旧表消失；各新增字段和迁移的数据保留由各自测试覆盖，不再因合法新增列误报。没有跳过测试或修改 CI 检查范围。

本机真实 MySQL 仍受环境限制，修复后的集成验证以 GitHub 最新 CI 为准；PR 中 Deploy production 跳过是工作流预期，不是部署失败。本修复不授权合并或部署。
