package common

import (
	"context"
	"fmt"
	"path"
	"runtime"

	"github.com/devopsext/utils"
	"github.com/opentracing/opentracing-go"
	opentracingLog "github.com/opentracing/opentracing-go/log"
)

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

func getCallerInfo(offset int) (string, string, int) {

	pc := make([]uintptr, 15)
	n := runtime.Callers(offset, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()

	function := getLastPath(frame.Function, 1)
	file := getLastPath(frame.File, 2)
	line := frame.Line

	return function, file, line
}

func startSpanFromContext(ctx context.Context, offset int, opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context) {

	operation, file, line := getCallerInfo(offset)

	span, context := opentracing.StartSpanFromContext(ctx, operation, opts...)
	if span != nil {

		span.SetTag("caller.file", file)
		span.SetTag("caller.line", line)
	}
	return span, context
}

func TracerStartSpan() opentracing.Span {

	span, _ := startSpanFromContext(context.Background(), 4)
	return span
}

func TracerStartSpanChildOf(spanContext opentracing.SpanContext) opentracing.Span {

	var span opentracing.Span
	if spanContext != nil {
		span, _ = startSpanFromContext(context.Background(), 4, opentracing.ChildOf(spanContext))
	} else {
		span, _ = startSpanFromContext(context.Background(), 4)
	}

	return span
}

func TracerStartSpanFollowsFrom(spanContext opentracing.SpanContext) opentracing.Span {

	var span opentracing.Span
	if spanContext != nil {
		span, _ = startSpanFromContext(context.Background(), 4, opentracing.FollowsFrom(spanContext))
	} else {
		span, _ = startSpanFromContext(context.Background(), 4)
	}

	return span
}

func TracerSpanError(span opentracing.Span, err error) {

	if span == nil {
		return
	}

	_, file, line := getCallerInfo(3)

	span.SetTag("error", true)
	span.LogFields(
		opentracingLog.Error(err),
		opentracingLog.String("line", fmt.Sprintf("%s:%d", file, line)),
	)
}
