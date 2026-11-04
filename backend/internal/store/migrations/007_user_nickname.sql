ALTER TABLE t_users ADD COLUMN nickname VARCHAR(64) CHARACTER SET utf8mb4 NOT NULL DEFAULT '' COMMENT '用户主动填写或选择的昵称，空字符串表示尚未补填' AFTER openid;
