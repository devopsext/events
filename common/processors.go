package common

import (
	"reflect"
)

type Processors struct {
	list []Processor
}

func (ps *Processors) Add(p Processor) {

	if reflect.ValueOf(p).IsNil() {
		return
	}
	ps.list = append(ps.list, p)
}

func (ps *Processors) Find(eventType string) Processor {
	for _, p := range ps.list {
		if p.EventType() == eventType {
			return p
		}
	}
	return nil
}

func (ps *Processors) FindHttpProcessor(eventType string) HttpProcessor {
	for _, p := range ps.list {
		hp, ok := p.(HttpProcessor)
		if ok && hp.EventType() == eventType {
			return hp
		}
	}
	return nil
}

func NewProcessors() *Processors {
	return &Processors{}
}
