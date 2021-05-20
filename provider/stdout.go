package provider

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/devopsext/events/common"
	"github.com/sirupsen/logrus"
)

type Stdout struct {
	callerInfo   bool
	callerOffset int
}

type templateFormatter struct {
	template        *template.Template
	timestampFormat string
}

func (f *templateFormatter) Format(entry *logrus.Entry) ([]byte, error) {

	r := entry.Message
	m := make(map[string]interface{})

	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			m[k] = v.Error()
		default:
			m[k] = v
		}
	}

	m["msg"] = entry.Message
	m["time"] = entry.Time.Format(f.timestampFormat)
	m["level"] = entry.Level.String()

	var err error

	if f.template != nil {

		var b bytes.Buffer
		err = f.template.Execute(&b, m)
		if err == nil {

			r = fmt.Sprintf("%s\n", b.String())
		}
	}

	return []byte(r), err
}

func (so *Stdout) trace(offset int) logrus.Fields {

	if so.callerInfo {

		function, file, line := common.GetCallerInfo(offset)
		return logrus.Fields{
			"file": file,
			"line": line,
			"func": function,
		}
	} else {
		return logrus.Fields{}
	}
}

func prepare(message string, args ...interface{}) string {

	if len(args) > 0 {
		return fmt.Sprintf(message, args...)
	} else {
		return message
	}
}

func exists(level logrus.Level, obj interface{}, args ...interface{}) (bool, string) {

	if obj == nil {
		return false, ""
	}

	message := ""

	switch v := obj.(type) {
	case error:
		message = v.Error()
	case string:
		message = v
	default:
		message = "not implemented"
	}

	flag := message != "" && logrus.IsLevelEnabled(level)
	if flag {
		message = prepare(message, args...)
	}
	return flag, message
}

func (so *Stdout) Info(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.InfoLevel, obj, args...); exists {
		logrus.WithFields(so.trace(3)).Infoln(message)
	}
}

func (so *Stdout) Warn(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.WarnLevel, obj, args...); exists {
		logrus.WithFields(so.trace(3)).Warnln(message)
	}
}

func (so *Stdout) Error(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.ErrorLevel, obj, args...); exists {
		logrus.WithFields(so.trace(3)).Errorln(message)
	}
}

func (so *Stdout) Debug(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.DebugLevel, obj, args...); exists {
		logrus.WithFields(so.trace(3)).Debugln(message)
	}
}

func (so *Stdout) Panic(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.PanicLevel, obj, args...); exists {
		logrus.WithFields(so.trace(3)).Panicln(message)
	}
}

func (so *Stdout) setStdout(format string, level string, templ string) {

	switch format {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	case "stdout":
		t, err := template.New("").Parse(templ)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.SetFormatter(&templateFormatter{template: t, timestampFormat: time.RFC3339})
	}

	switch level {
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)

	}

	logrus.SetOutput(os.Stdout)
}

func NewStdout(callerInfo bool, callerOffset int) *Stdout {

	return &Stdout{
		callerInfo:   callerInfo,
		callerOffset: callerOffset,
	}
}
