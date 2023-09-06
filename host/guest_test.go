package host

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/goext/slicex"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeMessage(t *testing.T) {
	msg := EncodeMessage(ControlMessage{Cmd: "stop", App: "example-server", MsgId: "abc"})
	assert.Equal(t, uint8(10), msg[len(msg)-1])
}

func TestDecodeMessage(t *testing.T) {
	msg := ControlMessage{Cmd: "stop", App: "example-server", MsgId: "abc"}
	data := EncodeMessage(msg)
	packet := []byte{}
	buf := make([]byte, 256)
	n := copy(buf, data)

	messages := DecodeMessage(&packet, buf[:n], false)

	assert.Equal(t, 1, len(messages))
	assert.Equal(t, msg, messages[0])
	assert.Equal(t, []byte{}, packet)
}

func TestDecodeMessageOverflow(t *testing.T) {
	msg := ControlMessage{Cmd: "stop", App: "example-server", MsgId: "abc"}
	data := EncodeMessage(msg)
	packet := []byte{}
	buf := make([]byte, 64)
	n := copy(buf, data)
	offset := 0

	messages := DecodeMessage(&packet, buf[:n], false)
	offset += 64

	assert.Equal(t, 0, len(messages))
	assert.Equal(t, buf, packet)

	for offset < len(data) {
		n = copy(buf, data[offset:])
		messages = DecodeMessage(&packet, buf[:n], false)
		offset += 64
	}

	assert.Equal(t, 1, len(messages))
	assert.Equal(t, msg, messages[0])
	assert.Equal(t, []byte{}, packet)
}

func TestDecodeMessageEof(t *testing.T) {
	msg := ControlMessage{Cmd: "stop", App: "example-server", MsgId: "abc"}
	data := slicex.Slice(EncodeMessage(msg), 0, -1)
	packet := []byte{}
	buf := make([]byte, 256)
	n := copy(buf, data)

	messages := DecodeMessage(&packet, buf[:n], true)

	assert.Equal(t, 1, len(messages))
	assert.Equal(t, msg, messages[0])
	assert.Equal(t, []byte{}, packet)
}

func TestGetSocketPath(t *testing.T) {
	cwd, _ := os.Getwd()
	sockFile, sockPath := GetSocketPath()

	assert.Equal(t, filepath.Join(cwd, "ngrpc.sock"), sockFile)

	if runtime.GOOS == "windows" {
		assert.Equal(t, "\\\\.\\pipe\\"+filepath.Join(cwd, "ngrpc.sock"), sockPath)
	} else {
		assert.Equal(t, filepath.Join(cwd, "ngrpc.sock"), sockPath)
	}
}

func TestIsHostOnline(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	assert.False(t, IsHostOnline())

	conf := goext.Ok(config.LoadConfig())
	host := NewHost(conf, false)
	goext.Ok(0, host.Start(false))
	defer host.Stop()

	assert.True(t, IsHostOnline())
}

func TestIsHostOnline_redundantSocketFile(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	sockFile, _ := GetSocketPath()
	os.WriteFile(sockFile, []byte{}, 0644)

	if runtime.GOOS != "windows" {
		assert.True(t, util.Exists(sockFile))
	}

	assert.False(t, IsHostOnline())

	if runtime.GOOS != "windows" {
		assert.False(t, util.Exists(sockFile))
	}
}

func TestNewGuest(t *testing.T) {
	app := config.App{
		Name: "example-server",
		Url:  "grpc://localhost:4000",
	}
	handleStop := func(msgId string) {}
	guest := NewGuest(app, handleStop)

	assert.Equal(t, app.Name, guest.AppName)
	assert.Equal(t, app.Url, guest.AppUrl)
	assert.Equal(t, 0, guest.state)
	assert.NotNil(t, guest.handleStopCommand)
}

func TestGuest_JoinAndLeave(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	cfg := goext.Ok(config.LoadConfig())
	host := NewHost(cfg, false)
	goext.Ok(0, host.Start(false))
	defer host.Stop()

	c := make(chan string)
	guest := NewGuest(config.App{
		Name: "example-server",
		Url:  "grpc://localhost:4000",
	}, func(msgId string) {
		c <- msgId
	})
	guest.Join()

	assert.Equal(t, 1, guest.state)
	assert.Equal(t, 1, len(host.clients))

	guest.Leave("app [example-server] stopped", "")

	time.Sleep(time.Millisecond * 10) // wait a while for the host to close the connection
	assert.Equal(t, 2, guest.state)
	assert.Equal(t, 0, len(host.clients))
}

func TestGuest_JoinRedundantSocketFile(t *testing.T) {
	sockFile, _ := GetSocketPath()
	os.WriteFile(sockFile, []byte{}, 0644)

	assert.True(t, util.Exists(sockFile))

	guest := NewGuest(config.App{
		Name: "example-server",
		Url:  "grpc://localhost:4000",
	}, func(msgId string) {})
	err := guest.connect()

	assert.NotNil(t, err)
	assert.False(t, util.Exists(sockFile))
}
