ALTER TABLE t_users ADD COLUMN avatar_url VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL DEFAULT NULL COMMENT '用户头像的随机读取路径，NULL表示尚未设置';
ALTER TABLE t_users ADD COLUMN avatar_data MEDIUMBLOB NULL COMMENT '规范化的256像素JPEG头像图片，随数据库持久保存';
ALTER TABLE t_users ADD UNIQUE INDEX uk_user_avatar_url (avatar_url);
