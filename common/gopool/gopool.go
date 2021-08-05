package gopool

import (
	"runtime"
	"time"

	"github.com/panjf2000/ants/v2"
)

var (
	defaultPool, _ = ants.NewPool(runtime.NumCPU(), ants.WithExpiryDuration(5*time.Second)) // block interval is 3
)

func Submit(task func()) error {
	return defaultPool.Submit(task)
}
