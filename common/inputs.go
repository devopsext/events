package common

import (
	"reflect"
	"sync"
)

type Inputs struct {
	list []Input
}

func (is *Inputs) Add(i Input) {

	if reflect.ValueOf(i).IsNil() {
		return
	}
	is.list = append(is.list, i)
}

func (is *Inputs) Start(wg *sync.WaitGroup, ots *Outputs) {

	for _, i := range is.list {

		if i != nil {
			(i).Start(wg, ots)
		}
	}
}

func NewInputs() *Inputs {
	return &Inputs{}
}
