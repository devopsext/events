package provider

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"

	"github.com/devopsext/events/common"
	"github.com/devopsext/utils"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DataDogOptions struct {
	TracerHost  string
	TracerPort  int
	ServiceName string
}

type DataDogTracerSpanContext struct {
	context ddtrace.SpanContext
}

type DataDogTracerSpan struct {
	span        ddtrace.Span
	spanContext *DataDogTracerSpanContext
	context     context.Context
	datadog     *DataDog
}

type DataDog struct {
	options      DataDogOptions
	callerOffset int
}

func (dds DataDogTracerSpan) GetContext() common.TracerSpanContext {
	if dds.span == nil {
		return nil
	}

	if dds.spanContext != nil {
		return dds.spanContext
	}

	dds.spanContext = &DataDogTracerSpanContext{
		context: dds.span.Context(),
	}
	return dds.spanContext
}

func (dds DataDogTracerSpan) SetCarrier(object interface{}) common.TracerSpan {

	if dds.span == nil {
		return nil
	}

	if reflect.TypeOf(object) != reflect.TypeOf(http.Header{}) {
		//log.Error(errors.New("Other than http.Header is not supported yet"))
		return dds
	}

	var h http.Header = object.(http.Header)
	tracer.Inject(dds.span.Context(), tracer.HTTPHeadersCarrier(h))
	/*if err != nil {
		log.Error(err)
	}*/
	return dds
}

func (dds DataDogTracerSpan) SetTag(key string, value interface{}) common.TracerSpan {

	if dds.span == nil {
		return nil
	}
	dds.span.SetTag(key, value)
	return dds
}

func (dds DataDogTracerSpan) Error(err error) common.TracerSpan {

	if dds.span == nil {
		return nil
	}

	dds.SetTag("error", true)
	return dds
}

func (dds DataDogTracerSpan) Finish() {
	if dds.span == nil {
		return
	}
	dds.span.Finish()
}

func (dd *DataDog) startSpanFromContext(ctx context.Context, offset int, opts ...tracer.StartSpanOption) (ddtrace.Span, context.Context) {

	operation, file, line := common.GetCallerInfo(offset)

	span, context := tracer.StartSpanFromContext(ctx, operation, opts...)
	if span != nil {
		span.SetTag("caller.line", fmt.Sprintf("%s:%d", file, line))
	}
	return span, context
}

func (dd *DataDog) startChildOfSpan(ctx context.Context, spanContext ddtrace.SpanContext) (ddtrace.Span, context.Context) {

	var span ddtrace.Span
	var context context.Context
	if spanContext != nil {
		span, context = dd.startSpanFromContext(ctx, dd.callerOffset+5, tracer.ChildOf(spanContext))
	} else {
		span, context = dd.startSpanFromContext(ctx, dd.callerOffset+5)
	}
	return span, context
}

func (dd *DataDog) StartSpan() common.TracerSpan {

	s, ctx := dd.startSpanFromContext(context.Background(), dd.callerOffset+4)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDog) getOpentracingSpanContext(object interface{}) ddtrace.SpanContext {

	h, ok := object.(http.Header)
	if ok {
		spanContext, err := tracer.Extract(tracer.HTTPHeadersCarrier(h))
		if err != nil {
			//log.Error(err)
			return nil
		}
		return spanContext
	}

	ddsc, ok := object.(*DataDogTracerSpanContext)
	if ok {
		return ddsc.context
	}
	return nil
}

func (dd *DataDog) StartChildSpan(object interface{}) common.TracerSpan {

	spanContext := dd.getOpentracingSpanContext(object)
	if spanContext == nil {
		return dd.StartSpan()
	}

	s, ctx := dd.startChildOfSpan(context.Background(), spanContext)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDog) StartFollowSpan(object interface{}) common.TracerSpan {
	spanContext := dd.getOpentracingSpanContext(object)
	if spanContext == nil {
		return dd.StartSpan()
	}

	s, ctx := dd.startChildOfSpan(context.Background(), spanContext)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDog) SetCallerOffset(offset int) {
	dd.callerOffset = offset
}

func startDataDogTracer(options DataDogOptions) {

	disabled := utils.IsEmpty(options.TracerHost)

	if disabled {
		return
	}

	addr := net.JoinHostPort(
		options.TracerHost,
		strconv.Itoa(options.TracerPort),
	)
	tracer.Start(tracer.WithAgentAddr(addr), tracer.WithServiceName(options.ServiceName))
}

func NewDataDog(options DataDogOptions) *DataDog {

	startDataDogTracer(options)

	return &DataDog{
		options:      options,
		callerOffset: 0,
	}
}
