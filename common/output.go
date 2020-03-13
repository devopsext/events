package common

type Output interface {
	Send(o interface{})
}
