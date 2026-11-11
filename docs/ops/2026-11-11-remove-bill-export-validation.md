# 移除账单导出验证记录

日期：2026-11-11。环境：macOS arm64、Node v26.13.0、Go 1.27.3。

## 改动范围

移除设置页导出入口与事件、小程序导出 API、CSV 文件写入分享和错误提示、后端 CSV 路由和处理器及专用查询分支。旧路径 `/api/v1/entries/export` 由现有账单 ID 路由返回 400 `invalid_entry`。移除专属测试并更新设置页与接口回归测试。现有账单、数据库结构和备份功能未修改。

设计见[移除设计](../superpowers/specs/2026-11-11-remove-bill-export-design.md)，执行步骤见[实施计划](../superpowers/plans/2026-11-11-remove-bill-export.md)。

## 实际红绿证据

- 功能修改前，`node --test miniprogram/tests/m1-pages.test.js` 退出 1：设置页仍有 `exportCSV`，预期 `undefined`，12 项通过、1 项失败。
- 后端首次测试遇到默认 Go 缓存异常（`net/http/httptest is not in std`），切换独立 `GOCACHE=/private/tmp/cost-export-removal-go-cache` 后解决；期间修正测试删除后遗留的未使用 import，并纠正一次在根目录执行 Go 命令的问题。这些环境/编译错误不作为红灯证据。
- 功能修改前，在 `backend/` 使用独立缓存运行 `go test ./internal/api -run '^TestBillExportRemoved$' -count=1` 退出 1：预期 400，实际 200 CSV。
- 修改后，以下检查全部退出 0。

| 检查 | 结果 |
|---|---|
| 根目录 `node --test 'miniprogram/tests/*.test.js'` | 121 项通过，0 失败 |
| 根目录 `node miniprogram/tests/validate.js` | JavaScript 语法、JSON、页面文件检查通过 |
| backend 中 `go test -race ./...` | 全部包通过 |
| backend 中 `go vet ./...` | 通过 |
| backend 中 `go build -o /private/tmp/cost-export-removal-api ./cmd/api` | 通过，产物在仓库外 |
| `git diff --check` | 通过 |

后端命令均使用上述独立 GOCACHE。本次未配置独立 MySQL 或 Redis 测试连接，相应外部服务集成测试不作为已验证；没有认证、SQL、迁移变更。

## 自查与未完成项

已自查路由、前端入口与处理器、专属测试清理、共享分页查询行为、当前文档和历史说明；未发现额外问题。检查差异，未新增凭证或仓库内构建产物。

未完成独立代码审查；当前环境未找到项目要求的 Superpowers 技能，不声称完整遵循技能流程。设计与计划在功能代码修改前写入，但在测试调整之后，未伪造技能评审或单独设计批准。

微信开发者工具编译、真实微信和真机检查均未执行。未提交、部署或发布小程序。上线需要同步部署后端和发布小程序，旧版小程序的导出调用会失败。
