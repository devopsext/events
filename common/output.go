package common

type Output interface {
	Send(event *Event)
	Name() string
}
