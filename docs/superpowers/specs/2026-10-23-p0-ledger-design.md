# F01–F04 核心记账功能设计

> 2026-11-11 更新：账单 CSV 导出已移除，以下相关内容保留为历史设计或验证记录；当前范围见[移除设计](2026-11-11-remove-bill-export-design.md)。

用户已要求按 requirements.md 实现四项 P0 的完整前后端。以下细节是实施选择，保留原生小程序、Go、MySQL 与现有视觉风格；不涉及线上部署或真实库迁移。

## 范围和规则
- 单家庭、CNY、Asia/Shanghai；整数分，金额上限沿用 100000000000 分。
- F01：备注包含匹配，跨月起止日期均含当天（数据库转为左闭右开）；分类、成员、类型、账户、金额闭区间；分页和全量过滤合计共用条件。
- F02：管理员创建、改名、图标、排序、停用/启用。单层分类，历史显示最新名称；种类不可修改。旧记录允许保留原停用分类，但新建或切换到停用分类禁止。
- F03：共享账户，管理员维护名称、类型、初始余额及停用。允许负余额；初始余额表示纳入本账本之前的余额，不绑定生效日期；历史账单默认 account_id=0（未指定），不影响账户余额。余额=初始余额+关联收入-支出+转入-转出，修改删除账单自动反映，无额外余额缓存。
- 转账独立记录，原子单行持久化后同时影响双方派生余额，不计入收支和预算；成员可新增并删除自己转账，管理员可删除全部。金额正数、不同账户、有效启用账户、请求幂等。暂不支持编辑转账（删除后重建）。
- F04：所有有家庭访问权成员可导出同等可见范围的收支 CSV，含账户和记录人。转账在账户页面单独展示，不混入收支导出。CSV 带 UTF-8 BOM，正确引用特殊字符并中和公式注入；有导出条数上限且明确报错不能静默截断。
- 自动备份：提供 Docker Compose 备份服务，每24小时一次、保留14天（可配）；私有权限、原子完成、失败状态/日志/健康检查；提供隔离恢复校验，不覆盖业务库。未实际启用前不得声称云端备份运行。

## 接口契约
保留现有接口兼容，所有新接口沿用认证、家庭成员权限。

Category: `{id,name,kind,icon,sort_order,disabled}`；GET /api/v1/categories 返回所有分类数组；POST 创建；PATCH /categories/{id} 更新 `{name,icon,sort_order,disabled}`（kind 可传但不得改变）。管理员写。

Account: `{id,name,type,opening_balance_cent,balance_cent,disabled}`；类型 cash/wechat/alipay/bank/other。GET /api/v1/accounts 返回数组；POST 创建 `{name,type,opening_balance_cent}`；PATCH /accounts/{id} 更新 `{name,type,opening_balance_cent,disabled}`。管理员写。

Entry 增加整数 account_id，0 表示未指定。旧客户端省略兼容为0。收支写权限不变。停用账户与分类类似，只允许编辑时保留原引用。

GET /api/v1/entries 参数：month 或 start_date/end_date (YYYY-MM-DD，日期区间优先)；q、kind、category_id、created_by、account_id、min_amount_cent、max_amount_cent；page 默认1，page_size 默认50最大100。无 month/日期时默认当前月。返回 `{items,total,page,page_size,summary:{income_cent,expense_cent,balance_cent}}`，保留 month 字段（适用时）。summary 是全部匹配结果，不是当前页。参数不合法返回400。成员筛选使用创建人编号；GET /api/v1/entry-authors 返回 `{id}` 数组，含历史创建人（0代表历史未指定），供所有家庭成员筛选。

GET /api/v1/entries/export 参数与查询一致（忽略分页），返回 CSV 附件；最多50000条，超出返回400 `export_too_large`。

Transfer: `{id,from_account_id,to_account_id,amount_cent,occurred_at,note,client_request_id,created_by}`。POST /api/v1/transfers，GET 同路径接受 page/page_size 返回 `{items,total,page,page_size}`；DELETE /transfers/{id} 按创建人/管理员授权。

## 数据与权限
增加显式可重跑004迁移：分类扩展字段、账户、账单可空账户关联、转账表及索引。维护旧数据及迁移版本；SQL过滤必须使用参数，金额和日期严格校验。幂等重试不重复影响余额；账户快照必须一致；分类/账户停用与写记录并发需要保证规则。

## 页面
账本页：保留月度卡片与列表，增加搜索/筛选/导出入口，分页加载、全量合计、成员与账户显示，编辑器增加账户。新页面“分类管理”“账户管理/转账”从设置进入，沿用卡片、圆角、暖色背景、收支色彩、底部编辑面板；错误、空态、加载、禁用与确认提示齐全。

## 验收
跨月组合查询、分页合计、边界日期与恶意关键词；分类停用历史保留；账户收支修改删除及转账幂等/原子余额；普通成员权限；CSV中文/换行/公式；备份失败不替换旧成功文件、保留策略与隔离恢复。运行Go race/vet/build、隔离MySQL迁移集成、前端逻辑与页面验证；真机与微信开发者工具未执行时明确记录。
