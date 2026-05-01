use shorturl;

create table `short_url_map` (
    `id` bigint unsigned not null auto_increment comment '主键',
    `create_at` datetime not null default current_timestamp comment '创建时间',
    `create_by` varchar(64) not null default '' comment '创建者',
    `is_del` tinyint unsigned not null default '0' comment '是否删除',
    `lurl` varchar(160) default null comment '长链接',
    `md5` char(32) default null comment '长链接md5',
    `surl` varchar(11) default null comment '短链接',
    primary key (`id`),
    index (`is_del`),
    unique (`lurl`),
    unique (`surl`)
) engine = INNODB default charset = utf8mb4 comment = '长链接映射表';