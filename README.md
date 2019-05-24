## exporter-plus

| exporter name | exporter version | support  |offical|metric path|
| ------ | ------ | ------ | ------ |------ |
| [redis exporter](https://github.com/oliver006/redis_exporter) | v0.32.0 | Y |N|/redis|
| [node exporter](https://github.com/prometheus/node_exporter) | v0.17.0 | Y|Y|/node|
| [mysqld exporter](https://github.com/prometheus/mysqld_exporter) | v0.11.0 | Y|Y|/mysql|
| [mongodb exporter](https://github.com/dcu/mongodb_exporter) | v1.0.0 | Y |N|/mongodb|
| [elastic exporter](https://github.com/justwatchcom/elasticsearch_exporter) | v1.0.2 | Y | N |/elastic|
| [kafka exporter](https://github.com/danielqsj/kafka_exporter) | v1.2.0 | Y | N |/kafka|
| [postgres_exporter](https://github.com/wrouesnel/postgres_exporter) | v0.4.7 | Y | N |/postgres|

## why need exporter-plus?
It is not that easy to manage too many exporters on one host.

## what we have done to the exporters?
Just add some micro modifications except for the main logic of each
exporter

## how to run the exporter-plus?
* go build exporter-plus.go 
* exporter-plus --mysql.switch=enable --redis.switch=enable --mongodb.switch=enable --elastic.switch=enable
  --kafka.switch=enable --postgres.switch=enable
  --redis.addr="0.0.0.0:6379," --redis.password="123qwe,"
  --redis.alias="测试," --redis.separator="," --redis.check-keys="aa"
  --mysqld.dsn="root:123qwe@(localhost:3309)/" --es.uri="http://localhost:9200" --kafka.server=kafka:9092
  --zookeeper.server=zookeeper:2181
  *`tips exporter-plus --help will list all usage of exporter-plus`*
  
  
## other config
* postgres_exporter: Remember to use postgres database name in the connection string
> ```
> export DATA_SOURCE_NAME="postgresql://postgres_exporter:123qwe@127.0.0.1:5432/postgres?sslmode=disable"
> ```