# 调用链 zipkin-go API

提供初始化方法`NewZipkinTracer`构造Zipkin tracer, 之后使用[zipkin-go](https://github.com/openzipkin/zipkin-go)的API创建`trace`及`span`。

应用/服务生成的`span`会被写到日志文件，供调用链系统的其他组件消费。日志文件的缺省位置为`/data/logs/trace.log`。
为了避免日志文件越写越大，把磁盘空间用完，可以使用`logrotate`滚动日志，使日志使用的空间受到约束。
日志滚动操作结束后，须要向服务发送信号，通知服务重新打开日志文件。使用的信号可以通过函数`NewZipkinTracer`的最后一个参数进行修改，如果传`nil`，会使用缺省信号`SIGUSR1`。

## 接入方法

初始化参见下面代码：

```go
package main

import (
	"github.com/leon-yc/gopher/qutracing"
)

func main() {
	// 参数依次为服务名，采样率，日志文件保存的位置，服务监听的地址，通知服务重新打开文件使用的信号
	tracer, closer := qutracing.NewZipkinTracer("service-name", 1.0, "./trace.log", "", nil)
	defer closer.Close()

	qutracing.SetGlobalZipkinTracer(tracer)

	// 其他代码
}
```

example2目录下提供了一个简单的使用例子，展示了`span`，以及子`span`的创建。
