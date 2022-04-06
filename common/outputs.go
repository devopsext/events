package common

import (
	"encoding/json"
	"reflect"

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

func (ots *Outputs) send(e *Event, exclude []Output) {

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
			if !utils.Contains(exclude, o) {
				o.Send(e)
			}
		} else {
			if ots.logger != nil {
				ots.logger.Warn("Output is not defined")
			}
		}
	}
}

func (ots *Outputs) Send(e *Event) {
	ots.send(e, []Output{})
}

func (ots *Outputs) SendExclude(e *Event, exclude []Output) {
	ots.send(e, exclude)
}

func NewOutputs(logger sreCommon.Logger) Outputs {
	return Outputs{
		logger: logger,
	}
}
