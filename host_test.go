package ngrpc_test

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/host"
	"github.com/stretchr/testify/assert"
)

func TestSpawnApp(t *testing.T) {
	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := cfg.Apps[0]
	pid := goext.Ok(host.SpawnApp(app, tsCfg))

	assert.NotEqual(t, -1, pid)
	assert.NotEqual(t, 0, pid)

	host.SendCommand("stop", "")
	time.Sleep(time.Second) // Host.Stop waited a while for message flushing, we wait here too
}

func TestSpawnApp_built(t *testing.T) {
	goext.Ok(0, exec.Command("go", "build", "-o", "entry/main", "entry/main.go").Run())
	defer os.Remove("entry/main")

	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := cfg.Apps[0]
	app.Entry = "entry/main"
	pid := goext.Ok(host.SpawnApp(app, tsCfg))

	assert.NotEqual(t, -1, pid)
	assert.NotEqual(t, 0, pid)

	host.SendCommand("stop", "")
	time.Sleep(time.Second) // Host.Stop waited a while for message flushing, we wait here too
}

func TestSpawnApp_invalidEntry(t *testing.T) {
	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := cfg.Apps[0]
	app.Entry = ""

	pid, err := host.SpawnApp(app, tsCfg)

	assert.Equal(t, 0, pid)
	assert.Equal(t, "entry file is not set", err.Error())
}
