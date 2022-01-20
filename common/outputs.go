package common

import (
	"encoding/json"
	"time"

	sreCommon "github.com/devopsext/sre/common"
)

type Outputs struct {
	timeFormat string
	list       []Output
	logger     sreCommon.Logger
}

func (ots *Outputs) Add(o Output) {

	if o == nil {
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

	if e.Time == "" {
		e.Time = time.Now().UTC().Format(ots.timeFormat)
	}

	if e.TimeNano == 0 {
		e.TimeNano = time.Now().UTC().UnixNano()
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

			(o).Send(e)
		} else {
			if ots.logger != nil {
				ots.logger.Warn("Output is not defined")
			}
		}
	}
}

func NewOutputs(timeFormat string, logger sreCommon.Logger) *Outputs {
	return &Outputs{
		timeFormat: timeFormat,
		logger:     logger,
	}
}
