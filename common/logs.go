package common

type Logs struct {
	loggers []Logger
}

func (ls *Logs) Info(obj interface{}, args ...interface{}) {
	for _, l := range ls.loggers {
		l.Info(obj, args...)
	}
}

func (ls *Logs) Warn(obj interface{}, args ...interface{}) {
	for _, l := range ls.loggers {
		l.Warn(obj, args...)
	}
}

func (ls *Logs) Error(obj interface{}, args ...interface{}) {
	for _, l := range ls.loggers {
		l.Error(obj, args...)
	}
}

func (ls *Logs) Debug(obj interface{}, args ...interface{}) {
	for _, l := range ls.loggers {
		l.Debug(obj, args...)
	}
}

func (ls *Logs) Panic(obj interface{}, args ...interface{}) {
	for _, l := range ls.loggers {
		l.Panic(obj, args...)
	}
}

func (ls *Logs) Register(l Logger) {
	if l != nil {
		ls.loggers = append(ls.loggers, l)
	}
}

func NewLogs() *Logs {
	return &Logs{}
}
