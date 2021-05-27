package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DataDogOptions struct {
	ServiceName string
}

type DataDogTracerOptions struct {
	Host        string
	Port        int
	Tags        string
	ServiceName string
	Version     string
}

type DataDogLoggerOptions struct {
	Host        string
	Port        int
	Tags        string
	ServiceName string
	Version     string
	Level       string
}

type DataDogTracerSpanContext struct {
	context ddtrace.SpanContext
}

type DataDogTracerSpan struct {
	span        ddtrace.Span
	spanContext *DataDogTracerSpanContext
	context     context.Context
	datadog     *DataDogTracer
}

type DataDogTracerLogger struct {
	logger common.Logger
}

type DataDogTracer struct {
	enabled      bool
	options      DataDogTracerOptions
	logger       common.Logger
	callerOffset int
}

type DataDogLogger struct {
	connection   *net.UDPConn
	stdout       *Stdout
	log          *logrus.Logger
	options      DataDogLoggerOptions
	callerOffset int
}

func (ddsc DataDogTracerSpanContext) GetTraceID() uint64 {

	if ddsc.context == nil {
		return 0
	}
	return ddsc.context.TraceID()
}

func (ddsc DataDogTracerSpanContext) GetSpanID() uint64 {

	if ddsc.context == nil {
		return 0
	}
	return ddsc.context.SpanID()
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
		dds.datadog.logger.Error(errors.New("Other than http.Header is not supported yet"))
		return dds
	}

	var h http.Header = object.(http.Header)
	err := tracer.Inject(dds.span.Context(), tracer.HTTPHeadersCarrier(h))
	if err != nil {
		dds.datadog.logger.Error(err)
	}
	return dds
}

func (dds DataDogTracerSpan) SetTag(key string, value interface{}) common.TracerSpan {

	if dds.span == nil {
		return nil
	}
	dds.span.SetTag(key, value)
	return dds
}

func (dds DataDogTracerSpan) SetBaggageItem(restrictedKey, value string) common.TracerSpan {
	if dds.span == nil {
		return nil
	}
	dds.span.SetBaggageItem(restrictedKey, value)
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

func (ddtl *DataDogTracerLogger) Log(msg string) {
	ddtl.logger.Info(msg)
}

func (dd *DataDogTracer) startSpanFromContext(ctx context.Context, offset int, opts ...tracer.StartSpanOption) (ddtrace.Span, context.Context) {

	operation, file, line := common.GetCallerInfo(offset)

	span, context := tracer.StartSpanFromContext(ctx, operation, opts...)
	if span != nil {
		span.SetTag("file", fmt.Sprintf("%s:%d", file, line))
	}
	return span, context
}

func (dd *DataDogTracer) startChildOfSpan(ctx context.Context, spanContext ddtrace.SpanContext) (ddtrace.Span, context.Context) {

	var span ddtrace.Span
	var context context.Context
	if spanContext != nil {
		span, context = dd.startSpanFromContext(ctx, dd.callerOffset+5, tracer.ChildOf(spanContext))
	} else {
		span, context = dd.startSpanFromContext(ctx, dd.callerOffset+5)
	}
	return span, context
}

func (dd *DataDogTracer) StartSpan() common.TracerSpan {

	s, ctx := dd.startSpanFromContext(context.Background(), dd.callerOffset+4)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDogTracer) StartSpanWithTraceID(traceID uint64) common.TracerSpan {

	opt := tracer.WithSpanID(traceID) // due to span ID equals trace ID if there is no parent
	s, ctx := dd.startSpanFromContext(context.Background(), dd.callerOffset+4, opt)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDogTracer) getOpentracingSpanContext(object interface{}) ddtrace.SpanContext {

	h, ok := object.(http.Header)
	if ok {
		spanContext, err := tracer.Extract(tracer.HTTPHeadersCarrier(h))
		if err != nil {
			dd.logger.Error(err)
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

func (dd *DataDogTracer) StartChildSpan(object interface{}) common.TracerSpan {

	spanContext := dd.getOpentracingSpanContext(object)
	if spanContext == nil {
		return nil
	}

	s, ctx := dd.startChildOfSpan(context.Background(), spanContext)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDogTracer) StartFollowSpan(object interface{}) common.TracerSpan {
	spanContext := dd.getOpentracingSpanContext(object)
	if spanContext == nil {
		return nil
	}

	s, ctx := dd.startChildOfSpan(context.Background(), spanContext)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDogTracer) SetCallerOffset(offset int) {
	dd.callerOffset = offset
}

func (dd *DataDogTracer) Enabled() bool {
	return dd.enabled
}

func (dd *DataDogLogger) Info(obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.InfoLevel, obj, args...); exists {
		dd.log.WithFields(fields).Infoln(message)
	}
	return dd
}

func (dd *DataDogLogger) SpanInfo(span common.TracerSpan, obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.InfoLevel, obj, args...); exists {
		fields = common.AddTracerFields(span, fields)
		dd.log.WithFields(fields).Infoln(message)
	}
	return dd
}

func (dd *DataDogLogger) Warn(obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.WarnLevel, obj, args...); exists {
		dd.log.WithFields(fields).Warnln(message)
	}
	return dd
}

func (dd *DataDogLogger) Error(obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.ErrorLevel, obj, args...); exists {
		dd.log.WithFields(fields).Errorln(message)
	}
	return dd
}

func (dd *DataDogLogger) SpanError(span common.TracerSpan, obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.ErrorLevel, obj, args...); exists {
		fields = common.AddTracerFields(span, fields)
		dd.log.WithFields(fields).Errorln(message)
	}
	return dd
}

func (dd *DataDogLogger) Debug(obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.DebugLevel, obj, args...); exists {
		dd.log.WithFields(fields).Debugln(message)
	}
	return dd
}

func (dd *DataDogLogger) SpanDebug(span common.TracerSpan, obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.DebugLevel, obj, args...); exists {
		fields = common.AddTracerFields(span, fields)
		dd.log.WithFields(fields).Debugln(message)
	}
	return dd
}

func (dd *DataDogLogger) Panic(obj interface{}, args ...interface{}) common.Logger {

	if exists, fields, message := dd.exists(logrus.PanicLevel, obj, args...); exists {
		dd.log.WithFields(fields).Panicln(message)
	}
	return dd
}

func (dd *DataDogLogger) Stack(offset int) common.Logger {
	dd.callerOffset = dd.callerOffset - offset
	return dd
}

func (dd *DataDogLogger) exists(level logrus.Level, obj interface{}, args ...interface{}) (bool, logrus.Fields, string) {

	message := ""

	switch v := obj.(type) {
	case error:
		message = v.Error()
	case string:
		message = v
	default:
		message = "not implemented"
	}

	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}

	if utils.IsEmpty(message) && !dd.log.IsLevelEnabled(level) {
		return false, nil, ""
	}

	function, file, line := common.GetCallerInfo(dd.callerOffset + 5)
	fields := logrus.Fields{
		"file":    fmt.Sprintf("%s:%d", file, line),
		"func":    function,
		"service": dd.options.ServiceName,
		"version": dd.options.Version,
	}
	return true, fields, message
}

func setDataDogTracerTags(opts []tracer.StartOption, sTags string) []tracer.StartOption {

	env := utils.GetEnvironment()
	pairs := strings.Split(sTags, ",")

	for _, p := range pairs {
		kv := strings.SplitN(p, "=", 2)
		k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])

		if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
			ed := strings.SplitN(v[2:len(v)-1], ":", 2)
			e, d := ed[0], ed[1]
			v = env.Get(e, "").(string)
			if v == "" && d != "" {
				v = d
			}
		}

		tag := tracer.WithGlobalTag(k, v)
		opts = append(opts, tag)
	}
	return opts
}

func startDataDogTracer(options DataDogTracerOptions, logger common.Logger, stdout *Stdout) bool {

	disabled := utils.IsEmpty(options.Host)
	if disabled {
		stdout.Debug("DataDog tracer is disabled.")
	}

	addr := net.JoinHostPort(
		options.Host,
		strconv.Itoa(options.Port),
	)

	var opts []tracer.StartOption
	opts = append(opts, tracer.WithAgentAddr(addr))
	opts = append(opts, tracer.WithServiceName(options.ServiceName))
	opts = append(opts, tracer.WithServiceVersion(options.Version))
	opts = append(opts, tracer.WithLogger(&DataDogTracerLogger{logger: logger}))

	opts = setDataDogTracerTags(opts, options.Tags)

	tracer.Start(opts...)
	return !disabled
}

func NewDataDogTracer(options DataDogTracerOptions, logger common.Logger, stdout *Stdout) *DataDogTracer {

	enabled := startDataDogTracer(options, logger, stdout)

	return &DataDogTracer{
		options:      options,
		callerOffset: 0,
		logger:       logger,
		enabled:      enabled,
	}
}

func NewDataDogLogger(options DataDogLoggerOptions, logger common.Logger, stdout *Stdout) *DataDogLogger {

	if utils.IsEmpty(options.Host) {
		stdout.Debug("DataDog logger is disabled.")
		return nil
	}

	address := fmt.Sprintf("%s:%d", options.Host, options.Port)
	serverAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		stdout.Error(err)
		return nil
	}

	connection, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		stdout.Error(err)
		return nil
	}

	formatter := &logrus.JSONFormatter{}
	formatter.TimestampFormat = time.RFC3339Nano

	log := logrus.New()
	log.SetFormatter(formatter)

	switch options.Level {
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	case "panic":
		log.SetLevel(logrus.PanicLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	log.SetOutput(connection)

	return &DataDogLogger{
		connection:   connection,
		stdout:       stdout,
		log:          log,
		options:      options,
		callerOffset: 0,
	}
}