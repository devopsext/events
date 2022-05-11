package common

import (
	"encoding/json"
	"reflect"
	"regexp"

	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
)

type Outputs struct {
	list   []Output
	logger sreCommon.Logger
}

func (ots *Outputs) Add(o Output) {

	if reflect.ValueOf(o).IsNil() {
		return
	}
	ots.list = append(ots.list, o)
}

func (ots *Outputs) send(e *Event, exclude []Output, pattern string) {

	if e == nil {
		if ots.logger != nil {
			ots.logger.Warn("Event is not found")
		}
		return
	}

	if utils.IsEmpty(pattern) {
		if ots.logger != nil {
			ots.logger.Warn("Patter is empty")
		}
		return
	}

	json, err := json.Marshal(e)
	if err != nil {
		if ots.logger != nil {
			ots.logger.Error(err)
		}
		return
	}

	if ots.logger != nil {
		ots.logger.Debug("Original event => %s", string(json))
	}

	for _, o := range ots.list {

		if o != nil {
			matched, err := regexp.MatchString(pattern, o.Name())
			if err != nil {
				if ots.logger != nil {
					ots.logger.Error(err)
				}
				continue
			}
			if !matched {
				continue
			}
			o.Send(e)
		} else {
			if ots.logger != nil {
				ots.logger.Warn("Output is not defined")
			}
		}
	}
}

func (ots *Outputs) Send(e *Event) {
	ots.send(e, []Output{}, ".*")
}

func (ots *Outputs) SendForward(e *Event, exclude []Output, pattern string) {
	ots.send(e, exclude, pattern)
}

func NewOutputs(logger sreCommon.Logger) Outputs {
	return Outputs{
		logger: logger,
	}
}
