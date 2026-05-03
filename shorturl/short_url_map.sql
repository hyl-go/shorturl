use shorturl;

create table `short_url_map` (
    `id` bigint unsigned not null auto_increment comment '主键',
    `create_at` datetime not null default current_timestamp comment '创建时间',
    `update_at` datetime default current_timestamp on update current_timestamp comment '更新时间',
    `create_by` varchar(64) not null default '' comment '创建者',
    `is_del` tinyint unsigned not null default '0' comment '是否删除',
    `lurl` varchar(160) default null comment '长链接',
    `md5` char(32) default null comment '长链接md5',
    `surl` varchar(128) default null comment '短链接',
    `expire_at` datetime default null comment '过期时间',
    `category` varchar(32) default null comment '链接分类',
    `safety_status` tinyint(1) default 0 comment '安全状态：0=安全 1=可疑 2=危险',
    `page_title` varchar(255) default null comment '页面标题（爬取）',
    `page_description` text default null comment '页面描述（爬取）',
    `ai_suggestions` json default null comment 'AI建议短链名称',
    primary key (`id`),
    index (`is_del`),
    index `idx_category` (`category`),
    index `idx_safety_status` (`safety_status`),
    unique (`lurl`),
    unique (`surl`)
) engine = INNODB default charset = utf8mb4 comment = '长链接映射表';