package host

import (
	"os"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/util"
	"github.com/stretchr/testify/assert"
)

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
	assert.Equal(t, 0, guest.state)
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
