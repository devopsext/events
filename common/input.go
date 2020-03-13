package common

import (
	"sync"
)

type Input interface {
	Start(wg *sync.WaitGroup, outputs *Outputs)
}
