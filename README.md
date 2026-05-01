# 短链接项目

### 什么是短链接
将一个长的url网址如:
https://go-zero.dev/zh-cn/getting-started/
转换为
hyl/1ly7k

### 需求背景
许多公司内部需要大量发送营销短信或者通知类的短信，需要一个短链接配合各部门使用

### 需求描述
输入一个长链接得到一个唯一的短网址
用户点击短网址可以正常跳转到对应的网址
网址可以长期使用

### 短链接生成方式
#### hash
使用hash函数对长链接进行hash，得到hash值作为短链接标识，但数据量大的时候会出现哈希冲突
#### 发号器/自增序列
每收到一个转链请求，就使用发号器生成递增的序号，然后该序号转换成62进制，最后拼接到短域名后得到短链接。为什么用62进制因为生成后比较短且都是由字母数字组成阅览器可以认识

### 生成model层
goctl model mysql datasource -url="root:123456@tcp(127.0.0.1:13307)/shorturl" -table="short_url_map" -dir="./model" -c

goctl model mysql datasource -url="root:123456@tcp(127.0.0.1:13307)/shorturl" -table="sequence" -dir="./model" -c

### 参数校验使用validator
go get github.com/go-playground/validator/v10

### 按照布隆过滤器
go get -u github.com/bits-and-blooms/bloom/v3
