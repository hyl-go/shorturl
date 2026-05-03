create database shorturl;
use shorturl;


create table `sequence`
(
    `id`        bigint(20) unsigned not null AUTO_INCREMENT,
    `stub`      varchar(1) not null,
    `timestamp` timestamp  not null default current_timestamp on update current_timestamp,
    primary key (`id`),
    unique key `idx_uniq_stub` (`stub`)
) engine=MyIsAm default charset=utf8 comment = '序号表';