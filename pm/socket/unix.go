//go:build !windows
// +build !windows

package socket

import (
	"net"
	"time"
)

func Listen(path string) (net.Listener, error) {
	return net.Listen("unix", path)
}

func DialTimeout(path string, duration time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", path, duration)
}
