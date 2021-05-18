package common

type TracerSpanContext interface {
}

type TracerSpan interface {
	GetContext() TracerSpanContext
	SetCarrier(object interface{}) TracerSpan
	SetTag(key string, value interface{}) TracerSpan
	LogFields(fields map[string]interface{}) TracerSpan
	Error(err error) TracerSpan
	Finish()
}

type Tracer interface {
	StartSpan() TracerSpan
	StartChildSpanFrom(object interface{}) TracerSpan
	StartFollowSpanFrom(object interface{}) TracerSpan
	Stop()
}
