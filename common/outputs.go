package common

import (
	"encoding/json"
	"time"
)

type Outputs struct {
	timeFormat string
	list       []*Output
}

func (ots *Outputs) Add(o *Output) {

	ots.list = append(ots.list, o)
}

func (ots *Outputs) Send(e *Event) {

	if e == nil {

		log.Warn("Event is not found")
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

		log.Error(err)
		return
	}

	log.Debug("Original event => %s", string(json))

	for _, o := range ots.list {

		if o != nil {

			(*o).Send(e)
		} else {
			log.Warn("Output is not defined")
		}
	}
}

func NewOutputs(timeFormat string) *Outputs {
	return &Outputs{
		timeFormat: timeFormat,
	}
}
