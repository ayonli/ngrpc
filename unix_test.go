//go:build !windows
// +build !windows

package ngrpc_test

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/ayonli/ngrpc"
	"github.com/stretchr/testify/assert"
)

func TestWaitForExit(t *testing.T) {
	app, _ := ngrpc.Start("user-server")

	go func() {
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	defer func() {
		if re := recover(); re != nil {
			assert.Equal(t, "unexpected call to os.Exit(0) during test", fmt.Sprint(re))
		}
	}()

	app.WaitForExit()
}
