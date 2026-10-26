ALTER TABLE t_entries ADD COLUMN deleted_at DATETIME(6) NULL COMMENT '软删除时间，UTC；NULL表示正常账单';
