//go:build windows
// +build windows

package socket

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

func Listen(path string) (net.Listener, error) {
	return winio.ListenPipe(path, &winio.PipeConfig{})
}

func DialTimeout(path string, duration time.Duration) (net.Conn, error) {
	second := time.Second
	return winio.DialPipe(path, &second)
}
