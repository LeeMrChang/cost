CREATE TABLE IF NOT EXISTS t_accounts (
 id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '账户编号',
 ledger_id BIGINT NOT NULL COMMENT '家庭账本编号',
 name VARCHAR(100) NOT NULL COMMENT '账户名称',
 type VARCHAR(16) NOT NULL COMMENT '账户类型',
 opening_balance_cent BIGINT NOT NULL DEFAULT 0 COMMENT '初始余额分',
 disabled BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否停用',
 FOREIGN KEY (ledger_id) REFERENCES t_ledgers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='家庭支付账户';
CREATE TABLE IF NOT EXISTS t_transfers (
 id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '转账编号',
 ledger_id BIGINT NOT NULL COMMENT '家庭账本编号',
 from_account_id BIGINT NOT NULL COMMENT '转出账户',
 to_account_id BIGINT NOT NULL COMMENT '转入账户',
 amount_cent BIGINT NOT NULL COMMENT '转账金额分',
 occurred_at DATETIME(6) NOT NULL COMMENT '转账时间UTC',
 note VARCHAR(500) NOT NULL DEFAULT '' COMMENT '转账备注',
 client_request_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '幂等请求编号',
 created_by BIGINT NULL COMMENT '记录人编号',
 UNIQUE KEY uk_transfer_request (ledger_id,client_request_id),
 KEY idx_transfer_time (ledger_id,occurred_at,id),
 FOREIGN KEY (ledger_id) REFERENCES t_ledgers(id),
 FOREIGN KEY (from_account_id) REFERENCES t_accounts(id),
 FOREIGN KEY (to_account_id) REFERENCES t_accounts(id),
 FOREIGN KEY (created_by) REFERENCES t_users(id),
 CHECK(amount_cent>0 AND amount_cent<=100000000000),
 CHECK(from_account_id<>to_account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='账户之间转账';
