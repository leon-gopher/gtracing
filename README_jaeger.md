# 调用链 OpenTracing API

提供初始化方法`NewJaegerTracer`构造tracer, 之后使用[opentracing-go](https://github.com/opentracing/opentracing-go)的API创建`trace`及`span`。

应用/服务生成的`span`会被写到日志文件，供调用链系统的其他组件消费。日志文件的缺省位置为`/data/logs/trace.log`。
为了避免日志文件越写越大，把磁盘空间用完，可以使用`logrotate`滚动日志，使日志使用的空间受到约束。
日志滚动操作结束后，须要通过`kill`向服务发送信号，通知服务重新打开日志文件。使用的信号可以通过函数`NewJaegerTracer`的`sig`参数进行修改，如果传`nil`，会使用缺省信号`SIGUSR1`。写文件默认使用非缓冲I/O，如果要用缓冲I/O，可以把`bufferSize`参数设置一个正值。

## 接入方法

初始化参见下面代码：

```go
package main

import (
	"github.com/opentracing/opentracing-go"

	"github.com/gopher/qutracing"
)

func main() {
	tracer, closer, err := qutracing.NewJaegerTracer("service-name", "jaeger.log", 1.0, nil, 0)
	opentracing.SetGlobalTracer(tracer)
	defer closer.Close()

	// 其他代码
}

```

这里的采样率设置成了1.0, 即对所有trace采样。如果设置的采样率小于0，系统会将其修正为0。修正后的
采样率如果大于1，tracer使用限速采样，每秒最多采样指定数量的trace。如果修正后的采样率介于0和1之
间则使用概率采样，采样指定比例的trace。

example3目录下提供了一个简单的使用例子，展示了`span`，以及子`span`的创建。

[OpenTracing语义约定](https://github.com/opentracing/specification/blob/master/semantic_conventions.md)列出了常用的span tag和log field名字，如`component`, `db.instance`, `db.type`, `http.url`, `peer.address`等。OpenTracing
API为常用tag/log field的设置提供了便利方法，避免了手工输入tag名字。

```go
//import "github.com/opentracing/opentracing-go/ext"

ext.Component.Set(span, "elasticsearch") // 效果等同于span.SetTag("component", "elasticsearch")
ext.DBInstance.Set(span, "users")
ext.DBType.Set(span, "redis")
ext.HTTPUrl.Set(span, "https://127.0.0.1:3000/users/")
ext.PeerAddress.Set(span, "127.0.0.1:3000")

ext.DBType.Set(span, "redis")
ext.DBInstance.Set(sapn, "r-2ze82aa4b574c714")
ext.DBStatement.Set(span, "GET key")
```

## 使用约定

### 通用规范

一，ServiceName

1. 一个项目使用相同的ServiceName。
2. ServiceName为项目代号，使用ASCII字符。如：ggs-leon-yc。
3. 长度不要超过64个字符。

二，Span单元

1. span的名称在一个ServiceName的内部需要唯一标识。
2. span名称由ASCII字符组成。如：controllerName/actionName,rpc/exServiceName,mysql_conn
3. 长度不要超过128个字符
4. 一个项目的span名称，不能超过2000个。
5. 一个调用链路里，除了第一个span，其它span都必须有父span。

### 调用链二期特别要求

一， 调用链二期要求每个span都要指定span.kind，其值可以是client, server, consumer或producer。可以通过下面的方法设置：

```go
// span.kind = client
ext.SpanKindRPCClient.Set(span)
// span.kind = server
ext.SpanKindRPCServer.Set(span)
// span.kind = consumer
ext.SpanKindRPCConsumer.Set(span)
// span.kind = producer
ext.SpanKindRPCProducer.Set(span)
```

二， span应当指定设置peer.service tag

```go
ext.PeerService.Set(span, "service B")
```

三， span操作名命名规范

数据库调用使用数据库名为前缀，词与词之间使用下划线连接。

例子如下：

设置span名称为：mysql_conn

设置span名称为：mysql_[操作名称]，如：mysql_insert,mysql_insertIgnore,mysql_insertMany,mysql_insertManyUpdate,mysql_insertUpdate,mysql_delete,mysql_update,mysql_select,mysql_query,mysql_exec等

设置span名称为：redis_conn

设置span名称为：redis_[操作名称]，如：redis_get,redis_mget,redis_del等

## tag的使用

参考[OpenTracing语义约定](https://github.com/opentracing/specification/blob/master/semantic_conventions.md), 页面的最后给出了常用操作RPC，消息队列，数据库，错误处理的约定。

### 数据库调用

```go
// redis
span := tracer.StartSpan("redis_set_mykey")
ext.SpanKindRPCClient.Set(span)
ext.DBType.Set(span, "redis")
ext.DBInstance.Set(span, "0")
ext.DBStatement.Set(span, "SET mykey 'WuValue'")
ext.PeerHostname.Set(span, "r-2zee16eb41e3b694.redis.rds.aliyuncs.com")
ext.PeerHostIPv4.SetString(span, "172.16.56.72")
ext.PeerPort.Set(span, 6379)
ext.PeerService.Set(span, "redis")
// 其他tag
```

```json
{
    "duration": 7,
    "operationName": "redis_set_mykey",
    "parentSpanID": "0",
    "process.serviceName": "my service2",
    "process.tags.hostname": "haus",
    "process.tags.ip": "192.168.94.174",
    "spanID": "54d9bd0945f1e333",
    "startTime": 1543834537398947,
    "tags.db.instance": "0",
    "tags.db.statement": "SET mykey 'WuValue'",
    "tags.db.type": "redis",
    "tags.peer.hostname": "",
    "tags.peer.ipv4": "",
    "tags.peer.port": "6379",
    "tags.peer.service": "redis",
    "tags.span.kind": "client",
    "traceID": ""
}
```

```go
// sql
span := tracer.StartSpan("mysql_query_user")
ext.SpanKindRPCClient.Set(span)
ext.DBType.Set(span, "sql")
ext.DBInstance.Set(span, "dbname")
ext.DBStatement.Set(span, "SELECT * FROM wuser_table")
ext.PeerHostname.Set(span, "")
ext.PeerHostIPv4.SetString(span, "")
ext.PeerPort.Set(span, 3306)
ext.PeerService.Set(span, "mysql")
ext.PeerAddress.Set(span, "")
// 其他tag
```

```json
{
    "duration": 8,
    "operationName": "mysql_query_user",
    "parentSpanID": "0",
    "process.serviceName": "my service2",
    "process.tags.hostname": "haus",
    "process.tags.ip": "",
    "spanID": "",
    "startTime": ,
    "tags.db.instance": "dbname",
    "tags.db.statement": "SELECT * FROM wuser_table",
    "tags.db.type": "sql",
    "tags.peer.address": "",
    "tags.peer.hostname": "",
    "tags.peer.ipv4": "172.16.56.72",
    "tags.peer.port": "3306",
    "tags.peer.service": "mysql",
    "tags.span.kind": "client",
    "traceID": ""
}
```

### 错误处理

span对应的操作失败了，设置error tag为true，然后通过log记录失败的细节。

```go
if file, err := os.Open("file-not-exist"); err != nil {
	ext.Error.Set(span, true)
	span.LogKV("event", "error", "error.kind", "internal error", "message", err.Error())
}
```

```json
{
    "duration": 20,
    "logs": [
        {
            "fields": [
                {
                    "key": "event",
                    "vStr": "error",
                    "vType": "string"
                },
                {
                    "key": "error.kind",
                    "vStr": "internal error",
                    "vType": "string"
                },
                {
                    "key": "message",
                    "vStr": "open file-not-exist: no such file or directory",
                    "vType": "string"
                }
            ],
            "timestamp": "2018-12-03T11:11:05.648272Z"
        }
    ],
    "operationName": "moon-landing",
    "parentSpanID": "0",
    "process.serviceName": "my service2",
    "process.tags.hostname": "haus",
    "process.tags.ip": "",
    "spanID": "1d368d8f7bc705d9",
    "startTime": ,
    "tags.error": "1",
    "tags.span.kind": "client",
    "traceID": "1d368d8f7bc705d9"
}
```

### 采样优先级

可以对单个`span`设置采样优先级，绕过采样率的限制。例如采样率小于1时，给`span`设置高优先级确保`span`被记录下来。

```go
span := tracer.StartSpan("important_operation")
ext.SamplingPriority.Set(span, 1)
```

### RPC

```Go
ext.SpanKindRPCClient.Set(span) // 客户端
//ext.SpanKindRPCServer.Set(span) // 服务端

// http://www.zypaas.com:9988
ext.PeerHostname.Set(span, "www.zypaas.com")
ext.PeerHostIPv4.SetString(span, "113.31.87.133")
ext.PeerPort.Set(span, 9988)

// B3头注入/提取参见example3/http.go
```

```json
{
    "duration": 759391,
    "operationName": "GET",
    "parentSpanID": "0",
    "process.serviceName": "service A",
    "process.tags.hostname": "haus",
    "process.tags.ip": "192.168.94.174",
    "spanID": "79ce088f4773a933",
    "startTime": 1543835662219090,
    "tags.http.method": "GET",
    "tags.http.url": "http://127.0.0.1:1234",
    "tags.msg": "client client client client client client client client client client ",
    "tags.peer.serivce": "service B",
    "tags.span.kind": "client",
    "traceID": "79ce088f4773a933"
}

{
    "duration": 759030,
    "operationName": "serve-request",
    "parentSpanID": "0",
    "process.serviceName": "service B",
    "process.tags.hostname": "haus",
    "process.tags.ip": "192.168.94.174",
    "spanID": "79ce088f4773a933",
    "startTime": 1543835662219282,
    "tags.msg": "server server server server server server server server server server ",
    "tags.span.kind": "server",
    "traceID": "79ce088f4773a933"
}
```

## B3头规范

* 主调方传递自己的TraceId,SpanId,ParentSpanId,Sampled共4个参数到被调方
* 被调方默认把参数中的SpanId作为ParentSpanId
* header头中的ParentSpanId在某些语言初始化时需要
* 如果为被采样记录，则头信息为空的，或者不传
* 如果头信息中的采样参数为0，或者未传递，则被调方使用自己的采样策略来记录调用链

b3头清单:

* x-b3-traceid
* x-b3-parentspanid
* x-b3-spanid
* x-b3-sampled
