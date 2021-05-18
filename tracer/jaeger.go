package tracer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"runtime"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/utils"
	"github.com/opentracing/opentracing-go"
	opentracingLog "github.com/opentracing/opentracing-go/log"
	"github.com/uber/jaeger-client-go"
	jaegerConfig "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

type JaegerOptions struct {
	ServiceName         string
	AgentHost           string
	AgentPort           int
	Endpoint            string
	User                string
	Password            string
	BufferFlushInterval int
	QueueSize           int
}

type JaegerSpanContext struct {
	context opentracing.SpanContext
}

type JaegerSpan struct {
	span        opentracing.Span
	spanContext *JaegerSpanContext
	context     context.Context
	jaeger      *Jaeger
}

type Jaeger struct {
	options JaegerOptions
	tracer  opentracing.Tracer
	list    []*JaegerSpan
}

func (js JaegerSpan) GetContext() common.TracerSpanContext {
	if js.span == nil {
		return nil
	}

	if js.spanContext != nil {
		return js.spanContext
	}

	js.spanContext = &JaegerSpanContext{
		context: js.span.Context(),
	}
	return js.spanContext
}

func (js JaegerSpan) SetCarrier(object interface{}) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	if reflect.TypeOf(object) != reflect.TypeOf(http.Header{}) {
		log.Error(errors.New("Other than http.Header is not supported yet"))
		return js
	}

	var h http.Header = object.(http.Header)
	js.jaeger.tracer.Inject(js.span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(h))
	return js
}

func (js JaegerSpan) SetTag(key string, value interface{}) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	js.span.SetTag(key, value)
	return js
}

func (js JaegerSpan) LogFields(fields map[string]interface{}) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	if len(fields) <= 0 {
		return js
	}

	var logFields []opentracingLog.Field

	for k, v := range fields {

		if v != nil {

			var logField opentracingLog.Field
			switch v.(type) {
			case bool:
				logField = opentracingLog.Bool(k, v.(bool))
			case int:
				logField = opentracingLog.Int(k, v.(int))
			case int64:
				logField = opentracingLog.Int64(k, v.(int64))
			case string:
				logField = opentracingLog.String(k, v.(string))
			case float32:
				logField = opentracingLog.Float32(k, v.(float32))
			case float64:
				logField = opentracingLog.Float64(k, v.(float64))
			case error:
				logField = opentracingLog.Error(v.(error))
			}

			logFields = append(logFields, logField)
		}
	}

	if len(logFields) > 0 {
		js.span.LogFields(logFields...)
	}
	return js
}

func (js JaegerSpan) Error(err error) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	_, file, line := js.jaeger.GetCallerInfo(3)

	js.SetTag("error", true)
	js.LogFields(map[string]interface{}{"error.message": err.Error(), "error.line": fmt.Sprintf("%s:%d", file, line)})
	return js
}

func (js JaegerSpan) Finish() {
	if js.span == nil {
		return
	}
	js.span.Finish()
}

func getLastPath(s string, limit int) string {

	index := 0
	dir := s
	var arr []string

	for !utils.IsEmpty(dir) {
		if index >= limit {
			break
		}
		index++
		arr = append([]string{path.Base(dir)}, arr...)
		dir = path.Dir(dir)
	}

	return path.Join(arr...)
}

func (j *Jaeger) GetCallerInfo(offset int) (string, string, int) {

	pc := make([]uintptr, 15)
	n := runtime.Callers(offset, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()

	function := getLastPath(frame.Function, 1)
	file := getLastPath(frame.File, 2)
	line := frame.Line

	return function, file, line
}

func (j *Jaeger) startSpanFromContext(ctx context.Context, offset int, opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context) {

	operation, file, line := j.GetCallerInfo(offset)

	span, context := opentracing.StartSpanFromContext(ctx, operation, opts...)
	if span != nil {
		span.SetTag("caller.line", fmt.Sprintf("%s:%d", file, line))
	}
	return span, context
}

func (j *Jaeger) startChildOfSpan(ctx context.Context, spanContext opentracing.SpanContext) (opentracing.Span, context.Context) {

	var span opentracing.Span
	var context context.Context
	if spanContext != nil {
		span, context = j.startSpanFromContext(ctx, 5, opentracing.ChildOf(spanContext))
	} else {
		span, context = j.startSpanFromContext(ctx, 5)
	}
	return span, context
}

func (j *Jaeger) startFollowsFromSpan(ctx context.Context, spanContext opentracing.SpanContext) (opentracing.Span, context.Context) {

	var span opentracing.Span
	var context context.Context
	if spanContext != nil {
		span, context = j.startSpanFromContext(ctx, 5, opentracing.FollowsFrom(spanContext))
	} else {
		span, context = j.startSpanFromContext(ctx, 5)
	}
	return span, context
}

func (j *Jaeger) StartSpan() common.TracerSpan {

	s, ctx := j.startSpanFromContext(context.Background(), 4)
	return JaegerSpan{
		span:    s,
		context: ctx,
		jaeger:  j,
	}
}

func (j *Jaeger) getOpentracingSpanContext(object interface{}) opentracing.SpanContext {

	h, ok := object.(http.Header)
	if ok {
		spanContext, err := j.tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(h))
		if err != nil {
			log.Error(err)
			return nil
		}
		return spanContext
	}

	sc, ok := object.(*JaegerSpanContext)
	if ok {
		return sc.context
	}
	return nil
}

func (j *Jaeger) StartChildSpanFrom(object interface{}) common.TracerSpan {

	spanContext := j.getOpentracingSpanContext(object)
	if spanContext == nil {
		return j.StartSpan()
	}

	s, ctx := j.startChildOfSpan(context.Background(), spanContext)
	return JaegerSpan{
		span:    s,
		context: ctx,
		jaeger:  j,
	}
}

func (j *Jaeger) StartFollowSpanFrom(object interface{}) common.TracerSpan {

	spanContext := j.getOpentracingSpanContext(object)
	if spanContext == nil {
		return j.StartSpan()
	}

	s, ctx := j.startFollowsFromSpan(context.Background(), spanContext)
	return JaegerSpan{
		span:    s,
		context: ctx,
		jaeger:  j,
	}
}

func newJaegerTracer(options JaegerOptions) opentracing.Tracer {

	disabled := utils.IsEmpty(options.AgentHost) && utils.IsEmpty(options.Endpoint)

	cfg := &jaegerConfig.Configuration{

		ServiceName: options.ServiceName,
		Disabled:    disabled,

		// Use constant sampling to sample every trace
		Sampler: &jaegerConfig.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},

		// Enable LogSpan to log every span via configured Logger
		Reporter: &jaegerConfig.ReporterConfig{
			LogSpans:            true,
			User:                options.User,
			Password:            options.Password,
			LocalAgentHostPort:  fmt.Sprintf("%s:%d", options.AgentHost, options.AgentPort),
			CollectorEndpoint:   options.Endpoint,
			BufferFlushInterval: time.Duration(options.BufferFlushInterval) * time.Second,
			QueueSize:           options.QueueSize,
		},
	}

	metricsFactory := prometheus.New()
	tracer, _, err := cfg.NewTracer(jaegerConfig.Metrics(metricsFactory), jaegerConfig.Logger(jaeger.StdLogger))
	if err != nil {
		log.Error(err)
		return opentracing.NoopTracer{}
	}
	opentracing.SetGlobalTracer(tracer)
	return tracer
}

func NewJaeger(options JaegerOptions) *Jaeger {

	return &Jaeger{
		options: options,
		tracer:  newJaegerTracer(options),
	}
}
