-- 009 分类图标升级（A 方案）：把 10 个默认分类的 emoji 换成区分度更高的图标。
-- 只更新「仍为旧默认值」的行，用户手动改过的图标一律跳过，绝不覆盖用户选择。
-- 身份匹配两路并用，缺一不可：
--   id  —— 覆盖「用户改过名」的情况（分类管理支持改名，如 夜宵 → 宵夜）；
--   name —— 覆盖「历史库经 005 以 MAX(id)+1 补种」的分类（这些行 id 不是 1~24）。
-- 旧值条件同时兜住两路的误伤：即便 id/name 命中同名的用户自建分类，图标不等于旧默认值就不会被改。
-- 含 U+FE0F 变体选择符的旧值（消费 🛍️）按两种写法并行比较，避免历史写入丢失变体符导致漏改。
UPDATE t_categories SET icon='🍗' WHERE ledger_id=1 AND kind='expense' AND (id=3 OR name='夜宵') AND (BINARY icon=BINARY '🍢');
UPDATE t_categories SET icon='🥐' WHERE ledger_id=1 AND kind='expense' AND (id=6 OR name='早餐') AND (BINARY icon=BINARY '🍳');
UPDATE t_categories SET icon='🍛' WHERE ledger_id=1 AND kind='expense' AND (id=7 OR name='午餐') AND (BINARY icon=BINARY '🍱');
UPDATE t_categories SET icon='🍲' WHERE ledger_id=1 AND kind='expense' AND (id=8 OR name='晚餐') AND (BINARY icon=BINARY '🍚');
UPDATE t_categories SET icon='🛒' WHERE ledger_id=1 AND kind='expense' AND (id=9 OR name='消费') AND (BINARY icon=BINARY '🛍️' OR BINARY icon=BINARY '🛍');
UPDATE t_categories SET icon='📦' WHERE ledger_id=1 AND kind='expense' AND (id=12 OR name='其他') AND (BINARY icon=BINARY '🧾');
UPDATE t_categories SET icon='🏆' WHERE ledger_id=1 AND kind='income' AND (id=14 OR name='奖金') AND (BINARY icon=BINARY '🎁');
UPDATE t_categories SET icon='⏰' WHERE ledger_id=1 AND kind='income' AND (id=15 OR name='加班') AND (BINARY icon=BINARY '🌙');
UPDATE t_categories SET icon='🎫' WHERE ledger_id=1 AND kind='income' AND (id=16 OR name='福利') AND (BINARY icon=BINARY '🧧');
UPDATE t_categories SET icon='🌱' WHERE ledger_id=1 AND kind='income' AND (id=20 OR name='副业') AND (BINARY icon=BINARY '📈');
