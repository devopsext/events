package common

type TracesSpanContext struct {
	contexts map[Tracer]TracerSpanContext
}

type TracesSpan struct {
	spans       map[Tracer]TracerSpan
	spanContext *TracesSpanContext
	traces      *Traces
	tracer      Tracer
}

type Traces struct {
	tracers []Tracer
}

func (tss TracesSpan) GetContext() TracerSpanContext {

	if len(tss.spans) <= 0 {
		return nil
	}

	if tss.spanContext != nil {
		return tss.spanContext
	}

	tss.spanContext = &TracesSpanContext{
		contexts: make(map[Tracer]TracerSpanContext),
	}

	for t, s := range tss.spans {

		ctx := s.GetContext()
		if ctx != nil {
			tss.spanContext.contexts[t] = ctx
		}
	}

	return tss.spanContext
}

func (tss TracesSpan) SetCarrier(object interface{}) TracerSpan {

	for _, s := range tss.spans {
		s.SetCarrier(object)
	}
	return tss
}

func (tss TracesSpan) SetTag(key string, value interface{}) TracerSpan {

	for _, s := range tss.spans {
		s.SetTag(key, value)
	}
	return tss
}

func (tss TracesSpan) Error(err error) TracerSpan {

	for _, s := range tss.spans {
		s.Error(err)
	}
	return tss
}

func (tss TracesSpan) Finish() {
	for _, s := range tss.spans {
		s.Finish()
	}
}

func (ts *Traces) Register(t Tracer) {
	if t != nil {
		ts.tracers = append(ts.tracers, t)
	}
}

func (ts *Traces) StartSpan() TracerSpan {

	if len(ts.tracers) <= 0 {
		return nil
	}

	span := TracesSpan{
		traces: ts,
		spans:  make(map[Tracer]TracerSpan),
	}

	for _, t := range ts.tracers {

		s := t.StartSpan()
		if s != nil {
			span.spans[t] = s
		}
	}
	return span
}

func (ts *Traces) StartChildSpan(object interface{}) TracerSpan {
	if len(ts.tracers) <= 0 {
		return nil
	}

	spanCtx, spanCtxOk := object.(*TracesSpanContext)

	span := TracesSpan{
		traces: ts,
		spans:  make(map[Tracer]TracerSpan),
	}

	for _, t := range ts.tracers {

		var s TracerSpan
		if spanCtxOk {
			s = t.StartChildSpan(spanCtx.contexts[t])
		} else {
			s = t.StartChildSpan(object)
		}
		if s != nil {
			span.spans[t] = s
		}
	}
	return span
}

func (ts *Traces) StartFollowSpan(object interface{}) TracerSpan {
	if len(ts.tracers) <= 0 {
		return nil
	}

	spanCtx, spanCtxOk := object.(*TracesSpanContext)

	span := TracesSpan{
		traces: ts,
		spans:  make(map[Tracer]TracerSpan),
	}

	for _, t := range ts.tracers {

		var s TracerSpan
		if spanCtxOk {
			s = t.StartFollowSpan(spanCtx.contexts[t])
		} else {
			s = t.StartFollowSpan(object)
		}
		if s != nil {
			span.spans[t] = s
		}
	}
	return span
}

func NewTraces() *Traces {
	return &Traces{}
}
