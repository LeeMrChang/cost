# 移除账单导出实施计划

依据：[设计说明](../specs/2026-11-11-remove-bill-export-design.md)。用户已要求直接移除功能，由当前代理执行，不调用子代理；适用技能未安装的缺口见设计说明。

- [x] 修改 `miniprogram/tests/m1-pages.test.js` 和 `backend/internal/api/p0_edges_test.go`，先验证设置页事件及旧导出接口的断言失败。
- [x] 删除 `miniprogram/pages/settings/index.{js,wxml}`、`miniprogram/utils/api.js` 中的导出代码，清理专属测试及测试桩。
- [x] 删除 `backend/internal/api/api.go`、`p0.go` 中的导出路由、处理器和查询特例，移除旧 CSV 测试，保留账单查询覆盖。
- [x] 同步 README、API 文档及文档索引，历史记录标注下线。
- [x] 执行 `node --test 'miniprogram/tests/*.test.js'`、`go test -race ./...`、`go vet ./...`、`go build -o /private/tmp/cost-export-removal-api ./cmd/api`，记录输出和跳过项。
- [x] 自查差异、密钥及生成产物；如未独立审查，明确记录；不提交、不部署。
