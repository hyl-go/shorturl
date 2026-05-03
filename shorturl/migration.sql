use shorturl;

ALTER TABLE `short_url_map`
  ADD COLUMN `expire_at` DATETIME DEFAULT NULL COMMENT '过期时间' AFTER `surl`,
  ADD COLUMN `category` VARCHAR(32) DEFAULT NULL COMMENT '链接分类（新闻/技术/购物/社交/视频/其他）',
  ADD COLUMN `safety_status` TINYINT(1) DEFAULT 0 COMMENT '安全状态：0=安全 1=可疑 2=危险',
  ADD COLUMN `page_title` VARCHAR(255) DEFAULT NULL COMMENT '页面标题（爬取）',
  ADD COLUMN `page_description` TEXT DEFAULT NULL COMMENT '页面描述（爬取）',
  ADD COLUMN `ai_suggestions` JSON DEFAULT NULL COMMENT 'AI 建议的短链名称列表',
  ADD COLUMN `update_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间';

CREATE INDEX idx_category ON short_url_map(category);
CREATE INDEX idx_safety_status ON short_url_map(safety_status);

CREATE TABLE IF NOT EXISTS `access_log` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `surl` VARCHAR(128) NOT NULL COMMENT '短链路径段',
  `access_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '访问时间',
  `ip` VARCHAR(45) DEFAULT NULL COMMENT '访问者 IP',
  `country` VARCHAR(64) DEFAULT NULL COMMENT '国家',
  `city` VARCHAR(64) DEFAULT NULL COMMENT '城市',
  `user_agent` VARCHAR(512) DEFAULT NULL COMMENT 'User-Agent',
  `device_type` VARCHAR(32) DEFAULT NULL COMMENT '设备类型',
  `os` VARCHAR(64) DEFAULT NULL COMMENT '操作系统',
  `browser` VARCHAR(64) DEFAULT NULL COMMENT '浏览器',
  `referer` VARCHAR(255) DEFAULT NULL COMMENT '来源页面',
  PRIMARY KEY (`id`),
  INDEX `idx_surl` (`surl`),
  INDEX `idx_access_time` (`access_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='访问日志表';

CREATE TABLE IF NOT EXISTS `access_stats` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `surl` VARCHAR(128) NOT NULL COMMENT '短链路径段',
  `date` DATE NOT NULL COMMENT '统计日期',
  `hour` TINYINT DEFAULT NULL COMMENT '小时（0-23）',
  `pv` INT UNSIGNED DEFAULT 0 COMMENT '页面浏览量',
  `uv` INT UNSIGNED DEFAULT 0 COMMENT '独立访客数',
  `device_mobile` INT UNSIGNED DEFAULT 0 COMMENT '移动端访问量',
  `device_desktop` INT UNSIGNED DEFAULT 0 COMMENT '桌面端访问量',
  `top_referer` VARCHAR(255) DEFAULT NULL COMMENT '主要来源',
  `top_country` VARCHAR(64) DEFAULT NULL COMMENT '主要国家',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_surl_date_hour` (`surl`, `date`, `hour`),
  INDEX `idx_date` (`date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='访问统计汇总表';

-- 统计查询性能（可选，线上建议执行）
-- CREATE INDEX idx_surl_access_time ON access_log (surl, access_time);

-- Demo：历史数据中分类为空的视为「其他」，便于管理员筛选统计（可重复执行）
UPDATE `short_url_map`
SET `category` = '其他'
WHERE (`category` IS NULL OR TRIM(`category`) = '')
  AND `is_del` = 0;
