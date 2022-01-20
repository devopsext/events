package common

import "sync"

type Inputs struct {
	list []Input
}

func (is *Inputs) Add(i Input) {

	if i == nil {
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
