//go:build !windows
// +build !windows

package host

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/util"
	"github.com/stretchr/testify/assert"
)

func TestHost_WaitForExit(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	config := goext.Ok(config.LoadConfig())
	host := NewHost(config, false)

	go func() {
		time.Sleep(time.Millisecond * 10) // wait a while for the host to start
		assert.Equal(t, 1, host.state)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	defer func() {
		if re := recover(); re != nil {
			assert.Equal(t, 0, host.state)
			assert.Equal(t, "unexpected call to os.Exit(0) during test", fmt.Sprint(re))
		}
	}()

	host.Start(true)
}
