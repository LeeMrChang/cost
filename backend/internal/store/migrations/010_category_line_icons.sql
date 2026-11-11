ALTER TABLE t_categories ADD COLUMN icon_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL DEFAULT NULL COMMENT '选用的系统线性图标key，NULL时显示Emoji';
ALTER TABLE t_categories ADD COLUMN system_icon_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL DEFAULT NULL COMMENT '内置分类只读角色，用于统一图标，用户自建为NULL';
