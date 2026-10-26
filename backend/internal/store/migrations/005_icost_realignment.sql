CREATE TABLE IF NOT EXISTS t_budgets_new (
  ledger_id BIGINT NOT NULL COMMENT '所属账本编号',
  scope ENUM('week','month','year') NOT NULL COMMENT '预算粒度：周week/月month/年year',
  period_key VARCHAR(16) NOT NULL COMMENT '期间键：周2026-W40、月2026-10、年2026',
  category_id BIGINT NOT NULL DEFAULT 0 COMMENT '分类编号，0表示总预算',
  amount_cent BIGINT NOT NULL COMMENT '预算金额，单位分，可为0',
  PRIMARY KEY (ledger_id,scope,period_key,category_id),
  FOREIGN KEY (ledger_id) REFERENCES t_ledgers(id),
  CHECK (amount_cent >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='账本预算（周/月/年三粒度，含总预算与分类预算）';
