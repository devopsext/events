package common

type Logger interface {
	Info(obj interface{}, args ...interface{})
	Warn(obj interface{}, args ...interface{})
	Error(obj interface{}, args ...interface{})
	Debug(obj interface{}, args ...interface{})
	Panic(obj interface{}, args ...interface{})
}
