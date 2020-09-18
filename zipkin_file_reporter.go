package gtracing

import (
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/reporter"
)

// defaults
const (
	defaultTimeout       = time.Second * 5 // timeout for http request in seconds
	defaultBatchInterval = time.Second * 1 // BatchInterval in seconds
	defaultBatchSize     = 100
	defaultMaxBacklog    = 1000
)

// fileReporter will send spans to a file.
type fileReporter struct {
	name          string
	file          *os.File
	encoder       *json.Encoder
	batchInterval time.Duration
	batchSize     int
	maxBacklog    int
	sendMtx       *sync.Mutex
	batchMtx      *sync.Mutex
	batch         []*model.SpanModel
	spanC         chan *model.SpanModel
	quit          chan struct{}
	shutdown      chan error
	signalC       chan os.Signal
	reopenSignal  os.Signal
}

// Send implements reporter
func (r *fileReporter) Send(s model.SpanModel) {
	r.spanC <- &s
}

// Close implements reporter
func (r *fileReporter) Close() error {
	close(r.quit)
	<-r.shutdown
	return r.file.Close()
}

func (r *fileReporter) loop() {
	var (
		nextSend   = time.Now().Add(r.batchInterval)
		ticker     = time.NewTicker(r.batchInterval / 10)
		tickerChan = ticker.C
	)
	defer ticker.Stop()

	for {
		select {
		case span := <-r.spanC:
			currentBatchSize := r.append(span)
			if currentBatchSize >= r.batchSize {
				nextSend = time.Now().Add(r.batchInterval)
				go func() {
					_ = r.sendBatch()
				}()
			}
		case <-tickerChan:
			if time.Now().After(nextSend) {
				nextSend = time.Now().Add(r.batchInterval)
				go func() {
					_ = r.sendBatch()
				}()
			}
		case <-r.quit:
			r.shutdown <- r.sendBatch()
			return
		case sig := <-r.signalC:
			if sig == r.reopenSignal {
				r.reopen()
			}
		}
	}
}

func (r *fileReporter) reopen() {
	r.sendMtx.Lock()
	defer r.sendMtx.Unlock()
	r.file.Close()
	file, _ := os.OpenFile(r.name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	r.file = file
	r.encoder = json.NewEncoder(r.file)
}

func (r *fileReporter) append(span *model.SpanModel) (newBatchSize int) {
	r.batchMtx.Lock()

	r.batch = append(r.batch, span)
	if len(r.batch) > r.maxBacklog {
		dispose := len(r.batch) - r.maxBacklog
		r.batch = r.batch[dispose:]
	}
	newBatchSize = len(r.batch)

	r.batchMtx.Unlock()
	return
}

func (r *fileReporter) sendBatch() error {
	// in order to prevent sending the same batch twice
	r.sendMtx.Lock()
	defer r.sendMtx.Unlock()

	// Select all current spans in the batch to be sent
	r.batchMtx.Lock()
	sendBatch := r.batch[:]
	r.batchMtx.Unlock()

	if len(sendBatch) == 0 {
		return nil
	}

	if err := r.encoder.Encode(sendBatch); err != nil {
		return err
	}

	// Remove sent spans from the batch even if they were not saved
	r.batchMtx.Lock()
	r.batch = r.batch[len(sendBatch):]
	r.batchMtx.Unlock()

	return nil
}

// ReporterOption sets a parameter for the HTTP Reporter
type ReporterOption func(r *fileReporter)

// BatchSize sets the maximum batch size, after which a collect will be
// triggered. The default batch size is 100 traces.
func BatchSize(n int) ReporterOption {
	return func(r *fileReporter) { r.batchSize = n }
}

// MaxBacklog sets the maximum backlog size. When batch size reaches this
// threshold, spans from the beginning of the batch will be disposed.
func MaxBacklog(n int) ReporterOption {
	return func(r *fileReporter) { r.maxBacklog = n }
}

// BatchInterval sets the maximum duration we will buffer traces before
// emitting them to the collector. The default batch interval is 1 second.
func BatchInterval(d time.Duration) ReporterOption {
	return func(r *fileReporter) { r.batchInterval = d }
}

func ReopenSignal(sig os.Signal) ReporterOption {
	return func(r *fileReporter) { r.reopenSignal = sig }
}

// NewFileReporter returns a new file Reporter.
func NewFileReporter(name string, opts ...ReporterOption) reporter.Reporter {
	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return reporter.NewNoopReporter()
	}

	r := &fileReporter{
		name:          name,
		file:          file,
		encoder:       json.NewEncoder(file),
		batchInterval: defaultBatchInterval,
		batchSize:     defaultBatchSize,
		maxBacklog:    defaultMaxBacklog,
		batch:         []*model.SpanModel{},
		spanC:         make(chan *model.SpanModel),
		quit:          make(chan struct{}, 1),
		shutdown:      make(chan error, 1),
		sendMtx:       &sync.Mutex{},
		batchMtx:      &sync.Mutex{},
		signalC:       make(chan os.Signal),
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.reopenSignal == nil {
		r.reopenSignal = DefaultSignal
	}
	signal.Notify(r.signalC, r.reopenSignal)

	go r.loop()

	return r
}
