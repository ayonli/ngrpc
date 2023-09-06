package pm

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/goext/collections"
	"github.com/ayonli/goext/mapx"
	"github.com/ayonli/goext/slicex"
	"github.com/ayonli/goext/stringx"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/pm/socket"
	"github.com/ayonli/ngrpc/util"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/rodaine/table"
	"github.com/struCoder/pidusage"
)

func init() {
	// The default logger writes log to the stderr, which is not intended for the CLI program, reset
	// it to the stdout instead.
	log.SetOutput(os.Stdout)
}

var openForAppend = os.O_CREATE | os.O_APPEND | os.O_WRONLY
var defaultTsOutDir = "node_modules/.ngrpc"

type appStat struct {
	app    string
	url    string
	pid    int
	uptime int
	memory float64
	cpu    float64
}

type clientRecord struct {
	conn      net.Conn
	App       string `json:"app"`
	Pid       int    `json:"pid"`
	StartTime int    `json:"startTime"`
}

type clientReading struct {
	conn    net.Conn
	packet  *[]byte
	bufRead []byte
	eof     bool
}

type messageRecord struct {
	conn net.Conn
	msg  ControlMessage
}

// The Host-Guest model is a mechanism used to hold communication between all apps running on
// the same machine.
//
// This mechanism is primarily used for the CLI tool sending control commands to the apps.
type Host struct {
	apps       []config.App
	tsCfg      config.TsConfig
	sockFile   string
	state      int
	standalone bool
	server     net.Listener
	clients    []clientRecord
	callbacks  *collections.Map[string, func(reply ControlMessage)]

	isProcessKeeper bool
	clientsLock     sync.RWMutex
	callbacksLock   sync.Mutex
}

func NewHost(conf config.Config, standalone bool) *Host {
	host := &Host{
		apps:          conf.Apps,
		state:         0,
		standalone:    standalone,
		server:        nil,
		clients:       []clientRecord{},
		callbacks:     collections.NewMap[string, func(reply ControlMessage)](),
		clientsLock:   sync.RWMutex{},
		callbacksLock: sync.Mutex{},
	}

	tsCfg, err := config.LoadTsConfig(conf.Tsconfig)

	if err == nil {
		host.tsCfg = tsCfg
	}

	return host
}

func (self *Host) Start(wait bool) error {
	sockFile, sockPath := GetSocketPath()
	listener, err := socket.Listen(sockPath)

	if err != nil {
		return err
	}

	self.state = 1
	self.server = listener
	self.sockFile = sockFile

	go func() {
		for {
			conn, err := listener.Accept()

			if err != nil {
				if self.state == 0 { // server has shut down
					break
				} else {
					continue
				}
			} else if self.state == 0 {
				break
			}

			go self.handleGuestConnection(conn)
		}
	}()

	if wait {
		self.waitForExit()
	}

	return nil
}

func (self *Host) Stop() {
	self.state = 0

	if len(self.clients) > 0 { // the :cli client may still be online
		for _, client := range self.clients {
			client.conn.Write(EncodeMessage(ControlMessage{Cmd: "goodbye", Fin: true}))
		}
	}

	if self.server != nil {
		if len(self.clients) > 0 {
			time.Sleep(time.Millisecond * 10) // wait a while for the message to be flushed
		}

		self.server.Close()
	}

	os.Remove(self.sockFile)

	if self.isProcessKeeper {
		os.Exit(0)
	}
}

func (self *Host) waitForExit() {
	self.isProcessKeeper = true
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	<-c
	self.Stop()
}

func (self *Host) addClient(client clientRecord) {
	self.clientsLock.Lock()
	self.clients = append(self.clients, client)
	self.clientsLock.Unlock()
}

func (self *Host) findClient(test func(client clientRecord) bool) (clientRecord, bool) {
	self.clientsLock.RLock()
	client, ok := slicex.Find(self.clients, func(item clientRecord, idx int) bool {
		return test(item)
	})
	self.clientsLock.RUnlock()
	return client, ok
}

func (self *Host) filterClients(test func(client clientRecord) bool) []clientRecord {
	self.clientsLock.RLock()
	clients := slicex.Filter(self.clients, func(item clientRecord, idx int) bool {
		return test(item)
	})
	self.clientsLock.RUnlock()
	return clients
}

func (self *Host) removeClient(test func(client clientRecord) bool) bool {
	self.clientsLock.Lock()
	count := len(self.clients)
	self.clients = slicex.Filter(self.clients, func(item clientRecord, idx int) bool {
		return !test(item)
	})
	ok := len(self.clients) < count
	self.clientsLock.Unlock()
	return ok
}

func (self *Host) setCallback(msgId string, fn func(reply ControlMessage)) {
	self.callbacksLock.Lock()
	self.callbacks.Set(msgId, fn)
	self.callbacksLock.Unlock()
}

func (self *Host) runCallback(msgId string, reply ControlMessage) {
	self.callbacksLock.Lock()
	fn, ok := self.callbacks.Get(msgId)

	if ok {
		fn(reply)
		self.callbacks.Delete(msgId)
	}

	self.callbacksLock.Unlock()
}

func (self *Host) handleGuestConnection(conn net.Conn) {
	packet := []byte{}
	buf := make([]byte, 256)

	for {
		if n, err := conn.Read(buf); err != nil {
			if errors.Is(err, io.EOF) {
				self.processGuestMessage(conn, &packet, buf[:n], true)
				self.handleGuestDisconnection(conn)
				break
			} else if errors.Is(err, net.ErrClosed) ||
				strings.Contains(err.Error(), "closed") { // go-winio error
				self.handleGuestDisconnection(conn)
				break
			}
		} else {
			self.processGuestMessage(conn, &packet, buf[:n], false)
		}
	}
}

func (self *Host) handleGuestDisconnection(conn net.Conn) {
	client, exists := self.findClient(func(item clientRecord) bool {
		return item.conn == conn
	})

	if !exists {
		return
	} else {
		self.clients = slicex.Filter[[]clientRecord](
			self.clients,
			func(item clientRecord, idx int) bool {
				return item.conn != conn
			},
		)
	}

	if client.App != "" && client.App != ":cli" && self.state == 1 && !self.standalone {
		// When the guest app is closed expectedly, it sends a `goodbye` command to the
		// host server and the server removes it normally. Otherwise, the connection is
		// closed due to program failure on the guest app, we can try to revive it.

		app, exists := slicex.Find(self.apps, func(item config.App, idx int) bool {
			return item.Name == client.App
		})

		if exists {
			time.Sleep(time.Second)

			if app.Stdout != "" {
				// Write the log to the app's log file instead, because the host daemon does not
				// have its own logger.
				file, err := os.OpenFile(app.Stdout, openForAppend, 0644)

				if err == nil {
					logger := log.New(file, "", log.LstdFlags)
					logger.Printf("app [%v] exited accidentally, reviving...", client.App)
				}

				file.Close()
			}

			SpawnApp(app, self.tsCfg)
		}
	}
}

func (self *Host) processGuestMessage(
	conn net.Conn,
	packet *[]byte,
	bufRead []byte,
	eof bool,
) {
	for _, msg := range DecodeMessage(packet, bufRead, eof) {
		self.handleMessage(conn, msg)
	}
}

func (self *Host) handleMessage(conn net.Conn, msg ControlMessage) {
	if msg.Cmd == "handshake" {
		self.handleHandshake(conn, msg)
	} else if msg.Cmd == "goodbye" {
		self.handleGoodbye(conn, msg)
	} else if msg.Cmd == "reply" {
		self.handleReply(conn, msg)
	} else if msg.Cmd == "stop" || msg.Cmd == "reload" {
		// When the host server receives a control command, it distribute the command to the target
		// app or all apps if the app is not specified.

		if msg.App != "" {
			client, exists := self.findClient(func(item clientRecord) bool {
				return item.App == msg.App && item.App != ":cli"
			})

			if exists {
				msgId, _ := gonanoid.New()

				client.conn.Write(EncodeMessage(ControlMessage{Cmd: msg.Cmd, MsgId: msgId}))
				self.setCallback(msgId, func(reply ControlMessage) {
					reply.Fin = true
					conn.Write(EncodeMessage(reply))
				})
			} else {
				conn.Write(EncodeMessage(ControlMessage{
					Cmd:   "reply",
					Error: fmt.Sprintf("app [%s] is not running", msg.App),
					Fin:   true,
				}))
			}
		} else {
			clients := self.filterClients(func(item clientRecord) bool {
				return item.App != ":cli"
			})

			if len(clients) > 0 {
				count := 0
				lock := sync.Mutex{}

				slicex.ForEach(clients, func(client clientRecord, _ int) {
					msgId, _ := gonanoid.New()

					client.conn.Write(EncodeMessage(ControlMessage{Cmd: msg.Cmd, MsgId: msgId}))
					self.callbacks.Set(msgId, func(reply ControlMessage) {
						lock.Lock()
						count++
						reply.Fin = count == len(clients)
						conn.Write(EncodeMessage(reply))
						lock.Unlock()
					})
				})
			} else { // this block is very unlikely to be hit, though
				conn.Write(EncodeMessage(ControlMessage{
					Cmd:   "reply",
					Error: "no app is running",
					Fin:   true,
				}))
			}
		}
	} else if msg.Cmd == "list" {
		clients := self.filterClients(func(item clientRecord) bool {
			return item.App != "" && item.App != ":cli"
		})
		conn.Write(EncodeMessage(ControlMessage{
			Cmd:    "reply",
			Guests: clients,
			Fin:    true,
		}))
	} else if msg.Cmd == "stop-host" {
		self.Stop()
	} else {
		conn.Write(EncodeMessage(ControlMessage{
			Cmd:   "reply",
			Error: "invalid message",
			Fin:   true,
		}))
	}
}

func (self *Host) handleHandshake(conn net.Conn, msg ControlMessage) {
	// After a guest establish the socket connection, it sends a `handshake` command indicates a
	// signing-in, we then store the client in the `hostClients` property for broadcast purposes.

	if msg.App != "" {
		self.addClient(clientRecord{
			conn:      conn,
			App:       msg.App,
			Pid:       msg.Pid,
			StartTime: int(time.Now().Unix()),
		})
	} else {
		self.addClient(clientRecord{
			conn:      conn,
			Pid:       msg.Pid,
			StartTime: int(time.Now().Unix()),
		})
	}

	conn.Write(EncodeMessage(ControlMessage{Cmd: "handshake"}))

	if msg.App != "" && msg.App != ":cli" {
		cli, ok := self.findClient(func(client clientRecord) bool {
			return client.App == ":cli"
		})

		if ok {
			cli.conn.Write(EncodeMessage(ControlMessage{
				Cmd: "online",
				App: msg.App,
				Pid: msg.Pid,
			}))
		}
	}
}

func (self *Host) handleGoodbye(conn net.Conn, msg ControlMessage) {
	self.removeClient(func(client clientRecord) bool {
		return client.conn == conn
	})

	if msg.Fin {
		conn.Close()
	}
}

func (self *Host) handleReply(conn net.Conn, msg ControlMessage) {
	// When a guest app finishes a control command, it send feedback via the `reply` command,
	// we use the `msgId` to retrieve the callback, run it and remove it.

	if msg.MsgId != "" {
		self.runCallback(msg.MsgId, msg)
	}

	if msg.Fin {
		conn.Close()
	}
}

// NOTE: this function runs in the CLI instead of the host server.
func (self *Host) startApp(appName string, guest *Guest) {
	conf, err := config.LoadConfig()

	if err != nil {
		fmt.Println(err)
		guest.Leave("", "")
		return
	}

	apps := []config.App{}
	start := func(app config.App) bool {
		_, err := SpawnApp(app, self.tsCfg)

		if err != nil {
			fmt.Printf("unable to start app [%s] (reason: %s)\n", app.Name, err)
			return false
		} else {
			return true
		}
	}

	if appName == "" {
		for _, app := range conf.Apps {
			if app.Serve {
				apps = append(apps, app)
			}
		}
	} else {
		app, exists := slicex.Find(conf.Apps, func(item config.App, idx int) bool {
			return item.Name == appName
		})

		if !exists {
			fmt.Printf("app [%s] doesn't exist in the config file\n", appName)
		} else if !app.Serve {
			fmt.Printf("app [%s] is not intended to be served\n", appName)
		} else {
			apps = append(apps, app)
		}
	}

	numStarted := 0

	if len(apps) != 0 {
		tsApp, ok := slicex.Find(apps, func(app config.App, _ int) bool {
			return filepath.Ext(app.Entry) == ".ts"
		})
		var err error

		if ok {
			outDir, _ := ResolveTsEntry(tsApp.Entry, self.tsCfg)
			err = CompileTs(self.tsCfg, outDir)
		}

		if err == nil {
			for _, app := range apps {
				ok := start(app)

				if ok {
					numStarted++
				}
			}
		}
	}

	if numStarted == 0 {
		guest.Leave("", "")
		return
	}

	waitChan := make(chan int)
	count := 0

	go func() {
		for {
			msg := <-guest.replyChan

			if msg.Cmd == "online" {
				log.Printf("app [%s] started (pid: %d)", msg.App, msg.Pid)
				count++

				if count == numStarted {
					break
				}
			}
		}

		waitChan <- 0
	}()

	<-waitChan
	guest.Leave("", "")
}

// NOTE: this function runs in the CLI instead of the host server.
func (self *Host) listApps(records []clientRecord) {
	var list []appStat

	for _, app := range self.apps {
		item, exists := slicex.Find(records, func(item clientRecord, idx int) bool {
			return item.App == app.Name
		})

		if exists {
			var memory float64
			var cpu float64
			sysInfo, err := pidusage.GetStat(item.Pid)

			if err != nil {
				memory = -1
				cpu = -1
			} else {
				memory = sysInfo.Memory
				cpu = sysInfo.CPU
			}

			list = append(list, appStat{
				app:    app.Name,
				url:    app.Url,
				pid:    item.Pid,
				uptime: int(time.Now().Unix()) - item.StartTime,
				memory: memory,
				cpu:    cpu,
			})
		} else if app.Serve {
			list = append(list, appStat{
				app:    app.Name,
				url:    app.Url,
				pid:    -1,
				uptime: -1,
				memory: -1,
				cpu:    -1,
			})
		}
	}

	tb := table.New("App", "URL", "Status", "Pid", "Uptime", "Memory", "CPU")

	for _, item := range list {
		parts := []any{item.app, item.url}

		if item.pid == -1 {
			parts = append(parts, "stopped", "N/A")
		} else {
			parts = append(parts, "running", fmt.Sprint(item.pid))
		}

		if item.uptime == -1 {
			parts = append(parts, "N/A")
		} else {
			parts = append(parts, time.Duration(int(time.Second)*item.uptime).String())
		}

		if item.memory == -1 {
			parts = append(parts, "N/A")
		} else {
			parts = append(parts, fmt.Sprintf("%.2f Mb", item.memory/1024/1024))
		}

		if item.cpu == -1 {
			parts = append(parts, "N/A")
		} else {
			parts = append(parts, fmt.Sprintf("%.2f %%", item.cpu))
		}

		tb.AddRow(parts...)
	}

	tb.Print()
}

// NOTE: this function runs in the CLI instead of the host server.
func (self *Host) sendCommand(cmd string, appName string) {
	guest := NewGuest(config.App{
		Name: ":cli",
		Url:  "",
	}, func(msgId string) {})
	guest.replyChan = make(chan ControlMessage)
	err := guest.connect()

	if err != nil {
		if cmd == "list" {
			self.listApps([]clientRecord{})
		}

		guest.Leave("", "")
		return
	}

	if cmd == "start" {
		self.startApp(appName, guest)
	} else if cmd == "restart" {
		self.sendAndWait(ControlMessage{Cmd: "stop", App: appName}, guest, false)
		self.startApp(appName, guest)
	} else {
		if cmd == "reload" {
			conf, err := config.LoadConfig()

			if err == nil {
				var hasTsApp bool
				var tsApp config.App

				if appName != "" {
					app, ok := slicex.Find(conf.Apps, func(app config.App, idx int) bool {
						return app.Name == appName
					})

					if ok && filepath.Ext(app.Entry) == ".ts" {
						tsApp = app
						hasTsApp = true
					}
				} else {
					tsApp, hasTsApp = slicex.Find(conf.Apps, func(app config.App, idx int) bool {
						return filepath.Ext(app.Entry) == ".ts"
					})
				}

				if hasTsApp {
					outDir, _ := ResolveTsEntry(tsApp.Entry, self.tsCfg)
					err = CompileTs(self.tsCfg, outDir)
				}
			}

			if err != nil {
				fmt.Println(err)
				guest.Leave("", "")
				return
			}
		}

		self.sendAndWait(ControlMessage{Cmd: cmd, App: appName}, guest, true)
	}
}

// NOTE: this function runs in the CLI instead of the host server.
func (self *Host) sendAndWait(msg ControlMessage, guest *Guest, fin bool) {
	guest.Send(msg)
	waitChan := make(chan int)

	go func() {
		for {
			msg := <-guest.replyChan

			if msg.Error != "" {
				log.Println(msg.Error)
			} else if msg.Text != "" {
				log.Println(msg.Text)
			} else if msg.Guests != nil {
				self.listApps(msg.Guests)
			}

			if msg.Fin {
				break
			}
		}

		waitChan <- 0
	}()

	<-waitChan

	if fin {
		if msg.Cmd == "stop" && msg.App == "" {
			// After all the apps have been stopped, stop the host server as well.
			self.sendAndWait(ControlMessage{Cmd: "stop-host"}, guest, true)
			time.Sleep(time.Microsecond * 20) // wait a while for the host to stop
			log.Println("host server shut down")
		} else {
			guest.Leave("", "")
		}
	}
}

func SendCommand(cmd string, appName string) {
	config, err := config.LoadConfig()

	if err != nil {
		fmt.Println(err)
		return
	}

	host := NewHost(config, true)
	host.sendCommand(cmd, appName)
}

func SpawnApp(app config.App, tsCfg config.TsConfig) (int, error) {
	return goext.Try(func() int {
		if app.Entry == "" {
			panic("entry file is not set")
		}

		var cmd *exec.Cmd
		entry, env := resolveApp(app, tsCfg)
		ext := filepath.Ext(entry)

		if ext == ".go" {
			cmd = exec.Command("go", "run", entry, app.Name)
		} else if ext == ".js" {
			cmd = exec.Command("node", "-r", "source-map-support/register", entry, app.Name)
		} else {
			cwd, _ := os.Getwd()
			cmd = exec.Command(filepath.Join(cwd, entry), app.Name)
		}

		if app.Stdout != "" {
			cmd.Stdout = goext.Ok(os.OpenFile(app.Stdout, openForAppend, 0644))
		}

		if app.Stderr != "" {
			cmd.Stderr = goext.Ok(os.OpenFile(app.Stderr, openForAppend, 0644))
		} else if app.Stdout != "" {
			cmd.Stderr = goext.Ok(os.OpenFile(app.Stdout, openForAppend, 0644))
		}

		if len(env) > 0 {
			cmd.Env = os.Environ()

			for key, value := range env {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}

		goext.Ok(0, cmd.Start())
		pid := cmd.Process.Pid
		goext.Ok(0, cmd.Process.Release())

		return pid
	})
}

func resolveApp(app config.App, tsCfg config.TsConfig) (entry string, env map[string]string) {
	if app.Entry == "" {
		panic("entry file is not set")
	}

	ext := filepath.Ext(app.Entry)
	env = map[string]string{}

	if ext == ".go" || ext == ".js" {
		entry = app.Entry
	} else if ext == ".ts" {
		entry = app.Entry
		outDir, outFile := ResolveTsEntry(entry, tsCfg)
		entry = outFile
		env["IMPORT_ROOT"] = outDir
	} else {
		entry = app.Entry
	}

	if app.Env != nil {
		env = mapx.Assign(env, app.Env)
	}

	return entry, env
}

func ResolveTsEntry(entry string, tsCfg config.TsConfig) (outDir string, outFile string) {
	if tsCfg.CompilerOptions.RootDir != "" {
		rootDir := filepath.Clean(tsCfg.CompilerOptions.RootDir)

		if rootDir != "" && rootDir != "." {
			_rootDir := rootDir + string(filepath.Separator)

			if strings.HasPrefix(entry, _rootDir) {
				entry = stringx.Slice(entry, len(_rootDir), len(entry))
			}
		}
	}

	if tsCfg.CompilerOptions.OutDir != "" {
		outDir = filepath.Clean(tsCfg.CompilerOptions.OutDir)

		if outDir != "" && outDir != "." {
			_outDir := outDir + string(filepath.Separator)

			if !strings.HasPrefix(entry, _outDir) {
				ext := filepath.Ext(entry)
				outFile = _outDir + stringx.Slice(entry, 0, -len(ext)) + ".js"
			}
		} else {
			outDir = ""
		}
	} else {
		outDir = defaultTsOutDir
		ext := filepath.Ext(entry)
		outFile = filepath.Join(outDir, stringx.Slice(entry, 0, -len(ext))+".js")
	}

	return outDir, outFile
}

func CompileTs(tsCfg config.TsConfig, outDir string) error {
	var cmd *exec.Cmd

	if outDir == "" {
		cmd = exec.Command("npx", "tsc")
	} else {
		if outDir != defaultTsOutDir && util.Exists(defaultTsOutDir) {
			os.RemoveAll(defaultTsOutDir) // remove redundant files
		}

		cmd = exec.Command("npx", "tsc", "--outDir", outDir)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil && tsCfg.CompilerOptions.NoEmitOnError {
		return err
	}

	return nil
}
