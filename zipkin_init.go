package gtracing

import (
	"io"
	"os"
	"time"

	zipkin "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/reporter"
)

var (
	zipkinTracer       *zipkin.Tracer
	noopZipkinTracer   *zipkin.Tracer
	noopZipkinReporter reporter.Reporter
)

func init() {
	noopZipkinReporter := reporter.NewNoopReporter()
	tracer, _ := zipkin.NewTracer(
		noopZipkinReporter,
		zipkin.WithNoopSpan(true),
		zipkin.WithSampler(zipkin.NeverSample),
		zipkin.WithSharedSpans(true),
	)
	noopZipkinTracer = tracer
}

func SetGlobalZipkinTracer(tracer *zipkin.Tracer) {
	zipkinTracer = tracer
}

func GlobalZipkinTracer() *zipkin.Tracer {
	if zipkinTracer == nil {
		return noopZipkinTracer
	}
	return zipkinTracer
}

func NewZipkinTracer(serviceName string, rate float64, traceFileName, hostPort string, sig os.Signal) (*zipkin.Tracer, io.Closer) {
	// set-up the local endpoint for our service
	endpoint, err := zipkin.NewEndpoint(serviceName, hostPort)
	if err != nil {
		return noopZipkinTracer, noopZipkinReporter
	}

	// set-up our sampling strategy
	sampler, err := zipkin.NewBoundarySampler(rate, time.Now().UnixNano())
	if err != nil {
		return noopZipkinTracer, noopZipkinReporter
	}

	if traceFileName == "" {
		traceFileName = "/data/logs/trace.log"
	}
	reporter := NewFileReporter(traceFileName, ReopenSignal(sig))

	// initialize the tracer
	tracer, err := zipkin.NewTracer(
		reporter,
		zipkin.WithLocalEndpoint(endpoint),
		zipkin.WithSampler(sampler),
		zipkin.WithTraceID128Bit(true),
	)
	if err != nil {
		reporter.Close()
		return noopZipkinTracer, noopZipkinReporter
	}

	return tracer, reporter
}
