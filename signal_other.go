// +build !windows

package gtracing

import (
	"os"
	"syscall"
)

var DefaultSignal os.Signal = syscall.SIGUSR1
