package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/goext/async"
	"github.com/ayonli/goext/slicex"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeMessage(t *testing.T) {
	msg := EncodeMessage(ControlMessage{Cmd: "stat", App: "example-server", MsgId: "abc"})
	assert.Equal(t, uint8(10), msg[len(msg)-1])
}

func TestDecodeMessage(t *testing.T) {
	msg := ControlMessage{Cmd: "stat", App: "example-server", MsgId: "abc"}
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
	msg := ControlMessage{Cmd: "stat", App: "example-server", MsgId: "abc"}
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
	msg := ControlMessage{Cmd: "stat", App: "example-server", MsgId: "abc"}
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
		assert.Equal(t, "\\\\?\\pipe\\"+filepath.Join(cwd, "ngrpc.sock"), sockPath)
	} else {
		assert.Equal(t, filepath.Join(cwd, "ngrpc.sock"), sockPath)
	}
}

func TestNewHost(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	config := goext.Ok(config.LoadConfig())
	host := NewHost(config, false)

	assert.Equal(t, config.Apps, host.apps)
	assert.Equal(t, 0, host.state)
	assert.Equal(t, []clientRecord{}, host.clients)
	assert.Equal(t, 0, host.callbacks.Size())
}

func TestHost_Start(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	config := goext.Ok(config.LoadConfig())
	host := NewHost(config, false)
	err := host.Start(false)
	defer host.Stop()

	assert.Nil(t, err)
	assert.Equal(t, 1, host.state)
	assert.NotNil(t, host.server)
	assert.Equal(t, filepath.Join(goext.Ok(os.Getwd()), "ngrpc.sock"), host.sockFile)

	host2 := NewHost(config, false)
	err2 := host2.Start(false)

	assert.Contains(t, err2.Error(), "address already in use")
	assert.Equal(t, 0, host2.state)
	assert.Nil(t, host2.server)
	assert.Equal(t, "", host2.sockFile)
}

func TestHost_Stop(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	config := goext.Ok(config.LoadConfig())
	host := NewHost(config, false)
	err := host.Start(false)

	assert.Nil(t, err)
	assert.True(t, util.Exists(host.sockFile))

	host.Stop()

	assert.Equal(t, 0, host.state)
	assert.False(t, util.Exists(host.sockFile))
}

func TestHost_WaitForExit(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	config := goext.Ok(config.LoadConfig())
	host := NewHost(config, false)

	go func() {
		time.Sleep(time.Second)
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

func TestIsLive(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	assert.False(t, IsLive())

	conf := goext.Ok(config.LoadConfig())
	host := NewHost(conf, false)
	goext.Ok(0, host.Start(false))
	defer host.Stop()

	assert.True(t, IsLive())
}

func TestIsLiv_redundantSocketFile(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	sockFile, _ := GetSocketPath()
	os.WriteFile(sockFile, []byte{}, 0644)

	assert.True(t, util.Exists(sockFile))

	assert.False(t, IsLive())

	assert.False(t, util.Exists(sockFile))
}

func TestSendCommand_stop(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	conf := goext.Ok(config.LoadConfig())
	host := NewHost(conf, false)
	goext.Ok(0, host.Start(false))
	defer host.Stop()

	c := make(chan string)
	guest := NewGuest(config.App{
		Name: "example-server",
		Uri:  "grpc://localhost:4000",
	}, func(msgId string) {
		c <- msgId
	})
	guest.Join()

	assert.Equal(t, 1, guest.state)
	assert.Equal(t, 1, len(host.clients))

	go func() {
		SendCommand("stop", "user-server")
		SendCommand("stop", "example-server")
	}()

	msgId := <-c
	guest.Leave("app [example-server] stopped", msgId)

	assert.NotEqual(t, "", msgId)
	assert.Equal(t, 0, guest.state)

	time.Sleep(time.Second)
	assert.Equal(t, 0, len(host.clients))
}

func TestSendCommand_stopAll(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	conf := goext.Ok(config.LoadConfig())
	host := NewHost(conf, false)
	goext.Ok(0, host.Start(false))
	defer host.Stop()

	c := make(chan []string)

	msgIds := []string{}
	push := async.Queue(func(msgId string) (fin bool) {
		msgIds = append(msgIds, msgId)
		fin = len(msgIds) == 2

		if fin {
			c <- msgIds
		}
		return fin
	})

	guest1 := NewGuest(config.App{
		Name: "example-server",
		Uri:  "grpc://localhost:4000",
	}, push)
	guest2 := NewGuest(config.App{
		Name: "user-server",
		Uri:  "grpc://localhost:4001",
	}, push)
	guest1.Join()
	guest2.Join()

	go func() {
		SendCommand("stop", "")
	}()

	<-c
	guest1.Leave("app [example-server] stopped", msgIds[0])
	guest2.Leave("app [user-server] stopped", msgIds[1])

	time.Sleep(time.Second)
	assert.Equal(t, 1, len(host.clients))
	assert.Equal(t, ":cli", host.clients[0].app)
}

func TestSendCommand_list(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	conf := goext.Ok(config.LoadConfig())
	host := NewHost(conf, false)
	goext.Ok(0, host.Start(false))
	defer host.Stop()

	c := make(chan []string)

	msgIds := []string{}
	push := async.Queue(func(msgId string) (fin bool) {
		msgIds = append(msgIds, msgId)
		fin = len(msgIds) == 2

		if fin {
			c <- msgIds
		}
		return fin
	})

	go func() {
		SendCommand("list", "")

		time.Sleep(time.Millisecond * 500)
		SendCommand("list", "")

		time.Sleep(time.Millisecond * 500)
		SendCommand("list", "")

		time.Sleep(time.Millisecond * 500)
		SendCommand("stop", "")
	}()

	time.Sleep(time.Millisecond * 500)
	guest1 := NewGuest(config.App{
		Name: "example-server",
		Uri:  "grpc://localhost:4000",
	}, push)
	guest1.Join()

	time.Sleep(time.Millisecond * 500)
	guest2 := NewGuest(config.App{
		Name: "user-server",
		Uri:  "grpc://localhost:4001",
	}, push)
	guest2.Join()

	<-c

	guest1.Leave("app [example-server] stopped", "")
	guest2.Leave("app [user-server] stopped", "")

	time.Sleep(time.Microsecond * 500)
}

func TestSendCommand_stopHost(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	conf := goext.Ok(config.LoadConfig())
	host := NewHost(conf, false)
	goext.Ok(0, host.Start(false))
	defer host.Stop()

	guest := NewGuest(config.App{
		Name: "example-server",
		Uri:  "grpc://localhost:4000",
	}, func(msgId string) {})
	guest.Join()

	assert.Equal(t, 1, guest.state)
	assert.Equal(t, 1, len(host.clients))

	go func() {
		SendCommand("stop-host", "")
	}()

	time.Sleep(time.Second)

	assert.Equal(t, 0, guest.state)
	assert.Equal(t, 0, host.state)
	assert.Equal(t, 1, len(host.clients))
	assert.Equal(t, ":cli", host.clients[0].app)
}

func TestCommand_listWhenNoHost(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	goext.Ok(0, util.CopyFile("../tsconfig.json", "tsconfig.json"))
	defer os.Remove("ngrpc.json")
	defer os.Remove("tsconfig.json")

	cmd := exec.Command("go", "run", "../cli/ngrpc/main.go", "list")
	out := string(goext.Ok(cmd.Output()))
	lines := strings.Split(out, "\n")

	assert.Equal(t,
		[]string{"App", "URI", "Status", "Pid", "Uptime", "Memory", "CPU"},
		strings.Fields(lines[0]))
	assert.Equal(t,
		[]string{"example-server", "grpc://localhost:4000", "stopped", "N/A", "N/A", "N/A", "N/A"},
		strings.Fields(lines[1]))
	assert.Equal(t,
		[]string{"user-server", "grpcs://localhost:4001", "stopped", "N/A", "N/A", "N/A", "N/A"},
		strings.Fields(lines[2]))
	assert.Equal(t,
		[]string{"post-server", "grpcs://localhost:4002", "stopped", "N/A", "N/A", "N/A", "N/A"},
		strings.Fields(lines[3]))
}
