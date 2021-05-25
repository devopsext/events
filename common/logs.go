package common

type Logs struct {
	loggers []Logger
}

func (ls *Logs) Info(obj interface{}, args ...interface{}) Logger {
	for _, l := range ls.loggers {
		l.Info(obj, args...)
	}
	return ls
}

func (ls *Logs) Warn(obj interface{}, args ...interface{}) Logger {
	for _, l := range ls.loggers {
		l.Warn(obj, args...)
	}
	return ls
}

func (ls *Logs) Error(obj interface{}, args ...interface{}) Logger {
	for _, l := range ls.loggers {
		l.Error(obj, args...)
	}
	return ls
}

func (ls *Logs) SpanError(span TracerSpan, obj interface{}, args ...interface{}) Logger {
	for _, l := range ls.loggers {
		l.SpanError(span, obj, args...)
	}
	return ls
}

func (ls *Logs) Debug(obj interface{}, args ...interface{}) Logger {
	for _, l := range ls.loggers {
		l.Debug(obj, args...)
	}
	return ls
}

func (ls *Logs) Panic(obj interface{}, args ...interface{}) Logger {
	for _, l := range ls.loggers {
		l.Panic(obj, args...)
	}
	return ls
}

func (ls *Logs) Stack(offset int) Logger {
	for _, l := range ls.loggers {
		l.Stack(offset)
	}
	return ls
}

func (ls *Logs) Register(l Logger) {
	if l != nil {
		ls.loggers = append(ls.loggers, l)
	}
}

func NewLogs() *Logs {
	return &Logs{}
}
