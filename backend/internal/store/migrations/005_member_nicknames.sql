ALTER TABLE t_users ADD COLUMN nickname VARCHAR(80) NOT NULL DEFAULT '' COMMENT '成员昵称，空字符串时使用成员编号';
