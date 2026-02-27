# ACRNM 爬虫

使用了外部接口 [acrnm-api](https://github.com/Drelf2018/acrnm-api)

这是一个通过解析 [acrnm.com/index](https://acrnm.com/index?sort=default&filter=txt) 页面并返回数据的接口。

### 使用

在可执行程序相同目录下创建配置文件 `config.ini` ，部分参数已脱敏，请自行填入。

```ini
csv = data.csv
logger = logs/2006-01-02.log
fangtang = xxx

[DingTalk]
name = ACRONYM
token = xxx
secret = xxx

[Qiniu]
access_key = xxx
secret_key = xxx
bucket_name = xxx
delete_after_days = 3
```