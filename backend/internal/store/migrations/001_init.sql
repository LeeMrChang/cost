CREATE TABLE IF NOT EXISTS ledgers (
 id BIGINT PRIMARY KEY,
 name VARCHAR(100) NOT NULL,
 currency CHAR(3) NOT NULL DEFAULT 'CNY',
 timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS categories (
 id BIGINT PRIMARY KEY,
 ledger_id BIGINT NOT NULL,
 name VARCHAR(100) NOT NULL,
 kind VARCHAR(10) NOT NULL,
 icon VARCHAR(32) NOT NULL,
 FOREIGN KEY (ledger_id) REFERENCES ledgers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS entries (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 ledger_id BIGINT NOT NULL,
 category_id BIGINT NOT NULL,
 kind VARCHAR(10) NOT NULL,
 amount_cent BIGINT NOT NULL,
 occurred_at DATETIME(6) NOT NULL,
 note VARCHAR(500) NOT NULL DEFAULT '',
 client_request_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
 UNIQUE KEY uk_request (ledger_id, client_request_id),
 KEY idx_ledger_time (ledger_id, occurred_at, id),
 FOREIGN KEY (ledger_id) REFERENCES ledgers(id),
 FOREIGN KEY (category_id) REFERENCES categories(id),
 CHECK (amount_cent > 0),
 CHECK (kind IN ('income', 'expense'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS budgets (
 ledger_id BIGINT NOT NULL,
 month CHAR(7) NOT NULL,
 amount_cent BIGINT NOT NULL,
 PRIMARY KEY (ledger_id, month),
 FOREIGN KEY (ledger_id) REFERENCES ledgers(id),
 CHECK (amount_cent >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
