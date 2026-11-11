# 分类候选图标接入计划

依据：[设计](../specs/2026-11-11-category-icons-design.md)。当前代理直接实施，保留已有未提交修改。

- [x] 在 domain/category_icons_test.go、api/category_icons_test.go、miniprogram/tests/p0-pages.test.js 添加并运行失败测试，覆盖默认映射、候选选择保存、009 迁移范围和数据保留。
- [x] 更新 domain/domain.go 的 10 个默认图标、categories/index.js 的去重候选列表。
- [x] 新增 store/migrations/009_category_icons.sql、store/category_icons.go 并在 mysql.go 串联事务迁移。
- [x] 运行前端全量测试与 validate.js，后端 go test -race ./...、go vet ./...、构建；通过 COST_TEST_MYSQL_ADMIN_DSN 仅创建随机测试库执行集成测试。
- [x] 自查差异、迁移和文档链接；更新 README、文档索引与验证记录，明确无独立审查、微信或生产部署证据。
