# 分类线性图标 API 与升级

## 数据

分类返回原有 icon（Emoji），另有 icon_key（空串表示 Emoji）与只读 system_icon_key（空串表示用户分类）。图标 key 与资源清单位于 `miniprogram/assets/category-icons/catalog.js`，同源后端副本由生成脚本更新。24 个默认映射 key 不重复；不同用户分类允许复用同一 key，数据库没有全局唯一约束。

GET /api/v1/categories 沿用现有认证、单用户单账本访问和缓存。预算接口返回分类 icon_key；统计页面按 category_id 关联分类接口，账单、搜索、日历、详情也通过分类集合显示图标。

## 创建与修改

POST /api/v1/categories 接受 name、kind、icon、icon_key、sort_order 等现有字段；icon_key 只能为空或已知清单值。客户端传入的 system_icon_key 被忽略，不能将自建分类伪装为内置。

PATCH /api/v1/categories/{id}：

| 提交 | 行为 |
|---|---|
| 不提交图标字段 | 保留现有选择，允许仅改名、排序、停用 |
| 只提交 icon | 更新 Emoji 并清空 icon_key，兼容旧客户端 |
| icon_key 为有效 key | 使用线稿，保留原 Emoji 回退；同时传 icon 时更新回退值 |
| icon_key 为 null 或空串 | 清空 key，使用保留的 Emoji |
| 未知 key 或非字符串 key | 400；不写入 |

icon_key 空值显式清空与字段缺省不同。system_icon_key 不在 PATCH 请求结构中，传入按未知字段拒绝；后续用户切换图标不改变它。分类改名不改变其稳定系统角色。

## 批量统一

POST /api/v1/categories/system-icons，无业务参数，沿用认证和当前账本上下文。仅将存在受信任 system_icon_key 的分类 icon_key 设为该角色 key；保留 icon 回退、名称、停用状态、排序、ID、账单关联；自建分类和其他账本不动。重复操作不会重复改变数据。返回 `{"updated":24}`（数量为实际修改行数）。事务失败全部回滚，成功后失效分类缓存。

小程序仅在用户点击“统一为系统图标”并接受二次确认后调用。忽略横幅不会发起请求，也不会启动弹窗。关闭状态按用户、账本、小程序版本和本轮功能版本保存。

## 显式迁移

009 保持 Notion §9 的 id OR name、kind、账本 1 与 BINARY 旧 icon 条件；消费支持有无 FE0F，保护自定义。010 新增可空 icon_key 与 system_icon_key，DDL 按列可重试，角色识别、旧默认值补 key 和版本标记在事务内提交。

010 对账本 1 的内置角色优先使用已知种子 ID；若该 ID 的名称明确属于另一默认分类则拒绝误认（旧库可能已有不同的 ID 分配）。找不到 ID 时只接受唯一同名历史行，歧义则保守跳过。只对 Emoji 为 A 前后已知默认值的行补线稿 key；自定义图标保持 Emoji。此方法不能在没有历史来源标记的情况下完美识别所有记录，尤其改名加 ID 冲突时会保守保留混排；可在编辑器逐个设置。用户自建同名行不会因前端 banner 而自动统一。

真实升级：停止旧后端、成功备份并确认目标版本，使用已有显式迁移流程升级至 010，再启动新版后端及编译/发布小程序。新服务使用新的分类缓存命名空间。**不在真实库删除 009 版本标记以强制重跑**；已发生的自定义覆盖无法凭空恢复，需检查备份或由用户重新设置。

本轮只在随机隔离数据库验证，未升级真实账本。素材、加载和测试状态见[验证记录](../ops/2026-11-11-category-line-icons-validation.md)。
