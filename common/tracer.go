package common

type TracerSpanContext interface {
	GetTraceID() string
	GetSpanID() string
}

type TracerSpan interface {
	GetContext() TracerSpanContext
	SetCarrier(object interface{}) TracerSpan
	SetTag(key string, value interface{}) TracerSpan
	Error(err error) TracerSpan
	SetBaggageItem(restrictedKey, value string) TracerSpan
	Finish()
}

type Tracer interface {
	StartSpan() TracerSpan
	StartChildSpan(object interface{}) TracerSpan
	StartFollowSpan(object interface{}) TracerSpan
}
