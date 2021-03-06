package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	sreCommon "github.com/devopsext/sre/common"
	utils "github.com/devopsext/utils"
)

var env = utils.GetEnvironment()
var log = utils.GetLog()

type TextTemplateOptions struct {
	TimeFormat string
}

type TextTemplate struct {
	template *template.Template
	options  TextTemplateOptions
	layout   string
	vars     interface{}
	logger   sreCommon.Logger
}

// replaceAll replaces all occurrences of a value in a string with the given
// replacement value.
func (tpl *TextTemplate) fReplaceAll(f, t, s string) (string, error) {
	return strings.Replace(s, f, t, -1), nil
}

// regexReplaceAll replaces all occurrences of a regular expression with
// the given replacement value.
func (tpl *TextTemplate) fRegexReplaceAll(re, pl, s string) (string, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return "", err
	}
	return compiled.ReplaceAllString(s, pl), nil
}

// regexMatch returns true or false if the string matches
// the given regular expression
func (tpl *TextTemplate) fRegexMatch(re, s string) (bool, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(s), nil
}

// toLower converts the given string (usually by a pipe) to lowercase.
func (tpl *TextTemplate) fToLower(s string) (string, error) {
	return strings.ToLower(s), nil
}

// toTitle converts the given string (usually by a pipe) to titlecase.
func (tpl *TextTemplate) fToTitle(s string) (string, error) {
	return strings.Title(s), nil
}

// toUpper converts the given string (usually by a pipe) to uppercase.
func (tpl *TextTemplate) fToUpper(s string) (string, error) {
	return strings.ToUpper(s), nil
}

// toJSON converts the given structure into a deeply nested JSON string.
func (tpl *TextTemplate) fToJSON(i interface{}) (string, error) {
	result, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(result)), err
}

// split is a version of strings.Split that can be piped
func (tpl *TextTemplate) fSplit(sep, s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}, nil
	}
	return strings.Split(s, sep), nil
}

// join is a version of strings.Join that can be piped
func (tpl *TextTemplate) fJoin(sep string, a []string) (string, error) {
	return strings.Join(a, sep), nil
}

func (tpl *TextTemplate) fIsEmpty(s string) (bool, error) {
	s1 := strings.TrimSpace(s)
	return len(s1) == 0, nil
}

func (tpl *TextTemplate) fGetEnv(key string) (string, error) {
	return env.Get(key, "").(string), nil
}

func (tpl *TextTemplate) fGetVar(key string) (string, error) {

	value := reflect.ValueOf(tpl.vars).FieldByName(key)
	if &value != nil {

		switch value.Kind() {
		case reflect.Int:
			return strconv.FormatInt(value.Int(), 10), nil
		default:
			return value.String(), nil
		}
	}
	return "", nil
}

func (tpl *TextTemplate) fTimeFormat(s string, format string) (string, error) {

	t, err := time.Parse(tpl.options.TimeFormat, s)
	if err != nil {

		return s, err
	}
	return t.Format(format), nil
}

func (tpl *TextTemplate) fJsonEscape(s string) (string, error) {

	bytes, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// toString converts the given value to string
func (tpl *TextTemplate) fToString(i interface{}) (string, error) {

	if i != nil {
		return fmt.Sprintf("%v", i), nil
	}
	return "", nil
}

func (tpl *TextTemplate) Execute(object interface{}) (*bytes.Buffer, error) {

	var b bytes.Buffer
	var err error

	if empty, _ := tpl.fIsEmpty(tpl.layout); empty {

		err = tpl.template.Execute(&b, object)
	} else {
		err = tpl.template.ExecuteTemplate(&b, tpl.layout, object)
	}

	if err != nil {

		tpl.logger.Error(err)
		return nil, err
	}
	return &b, nil
}

func NewTextTemplate(name string, fileOrVar string, options TextTemplateOptions, vars interface{}, logger sreCommon.Logger) *TextTemplate {

	var tpl = TextTemplate{}

	var t *template.Template
	var err1 error

	funcs := template.FuncMap{
		"regexReplaceAll": tpl.fRegexReplaceAll,
		"regexMatch":      tpl.fRegexMatch,
		"replaceAll":      tpl.fReplaceAll,
		"toLower":         tpl.fToLower,
		"toTitle":         tpl.fToTitle,
		"toUpper":         tpl.fToUpper,
		"toJSON":          tpl.fToJSON,
		"split":           tpl.fSplit,
		"join":            tpl.fJoin,
		"isEmpty":         tpl.fIsEmpty,
		"getEnv":          tpl.fGetEnv,
		"getVar":          tpl.fGetVar,
		"timeFormat":      tpl.fTimeFormat,
		"jsonEscape":      tpl.fJsonEscape,
		"toString":        tpl.fToString,
	}

	if utils.IsEmpty(fileOrVar) {
		logger.Warn("Template %s is empty.", name)
		return nil
	}

	if _, err := os.Stat(fileOrVar); err == nil {

		content, err := ioutil.ReadFile(fileOrVar)
		if err != nil {
			logger.Error(err)
			return nil
		}

		t, err1 = template.New(name).Funcs(funcs).Parse(string(content))
	} else {

		t, err1 = template.New(name).Funcs(funcs).Parse(fileOrVar)
	}

	if err1 != nil {
		logger.Error(err1)
		return nil
	}

	tpl.template = t
	tpl.options = options
	tpl.layout = name
	tpl.vars = vars
	tpl.logger = logger

	return &tpl
}
