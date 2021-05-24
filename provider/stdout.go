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

type StdoutOptions struct {
	Format   string
	Level    string
	Template string
}

type Stdout struct {
	log          *logrus.Logger
	options      StdoutOptions
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

	function, file, line := common.GetCallerInfo(so.callerOffset + offset)
	return logrus.Fields{
		"file": fmt.Sprintf("%s:%d", file, line),
		"func": function,
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
		so.log.WithFields(so.trace(3)).Infoln(message)
	}
}

func (so *Stdout) Warn(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.WarnLevel, obj, args...); exists {
		so.log.WithFields(so.trace(3)).Warnln(message)
	}
}

func (so *Stdout) Error(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.ErrorLevel, obj, args...); exists {
		so.log.WithFields(so.trace(3)).Errorln(message)
	}
}

func (so *Stdout) Debug(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.DebugLevel, obj, args...); exists {
		so.log.WithFields(so.trace(3)).Debugln(message)
	}
}

func (so *Stdout) Panic(obj interface{}, args ...interface{}) {

	if exists, message := exists(logrus.PanicLevel, obj, args...); exists {
		so.log.WithFields(so.trace(3)).Panicln(message)
	}
}

func newLog(options StdoutOptions) *logrus.Logger {

	log := logrus.New()

	switch options.Format {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		log.SetFormatter(&logrus.TextFormatter{})
	case "template":
		t, err := template.New("").Parse(options.Template)
		if err != nil {
			log.Panic(err)
		}
		log.SetFormatter(&templateFormatter{template: t, timestampFormat: time.RFC3339})
	default:
		log.SetFormatter(&logrus.TextFormatter{})
	}

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

	log.SetOutput(os.Stdout)
	return log
}

func (so *Stdout) SetCallerOffset(offset int) {
	so.callerOffset = offset
}

func NewStdout(options StdoutOptions) *Stdout {

	log := newLog(options)

	return &Stdout{
		log:          log,
		options:      options,
		callerOffset: 0,
	}
}
