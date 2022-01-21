package common

import (
	"encoding/json"
	"reflect"

	sreCommon "github.com/devopsext/sre/common"
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

func (ots *Outputs) Send(e *Event) {

	if e == nil {
		if ots.logger != nil {
			ots.logger.Warn("Event is not found")
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
			o.Send(e)
		} else {
			if ots.logger != nil {
				ots.logger.Warn("Output is not defined")
			}
		}
	}
}

func NewOutputs(logger sreCommon.Logger) *Outputs {
	return &Outputs{
		logger: logger,
	}
}
