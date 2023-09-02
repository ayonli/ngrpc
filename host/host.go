package host

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
	"github.com/ayonli/ngrpc/util"
	"github.com/rodaine/table"
)

type clientRecord struct {
	conn net.Conn
	app  string
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
	apps      []config.App
	tsCfg     config.TsConfig
	sockFile  string
	state     int
	server    net.Listener
	clients   []clientRecord
	callbacks *collections.Map[string, func(reply ControlMessage)]

	isProcessKeeper bool
	clientsLock     sync.RWMutex
	callbacksLock   sync.Mutex
}

func NewHost(conf config.Config) *Host {
	host := &Host{
		apps:          conf.Apps,
		state:         0,
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
	listener, err := net.Listen("unix", sockPath)

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
					log.Println(err)
					continue
				}
			} else if self.state == 0 {
				break
			}

			go self.handleGuestConnection(conn)
		}
	}()

	log.Printf("host server started (pid: %d)", os.Getpid())
	// <-self.clientsLock
	// <-self.callbacksLock

	if wait {
		self.waitForExit()
	}

	return nil
}

func (self *Host) Stop() {
	self.state = 0

	log.Println("host server shuting down")

	if self.clients != nil {
		for _, client := range self.clients {
			client.conn.Write(EncodeMessage(ControlMessage{Cmd: "goodbye", Fin: true}))
		}
	}

	if self.server != nil {
		time.Sleep(time.Second) // wait a while for the message to be flushed
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
		// self.pushGuestReading()

		if n, err := conn.Read(buf); err != nil {
			if errors.Is(err, io.EOF) {
				// self.pushClientReading(clientReading{
				// 	conn:    conn,
				// 	packet:  &packet,
				// 	bufRead: buf[:n],
				// 	eof:     true,
				// })
				// self.pushClientReading(clientReading{
				// 	conn: conn,
				// 	eof:  true,
				// })

				self.processGuestMessage(conn, &packet, buf[:n], true)
				self.handleGuestDisconnection(conn)
				// self.pushDisconnection(conn)
				break
			} else if errors.Is(err, net.ErrClosed) {
				self.handleGuestDisconnection(conn)
				// self.pushDisconnection(conn)
				// self.pushClientReading(clientReading{
				// 	conn: conn,
				// 	eof:  false,
				// })
				break
			}
		} else {
			// self.pushClientReading(clientReading{
			// 	conn:    conn,
			// 	packet:  &packet,
			// 	bufRead: buf[:n],
			// 	eof:     false,
			// })
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

	if client.app != "" && client.app != ":cli" && self.state == 1 {
		// When the guest app is closed expectedly, it sends a `goodbye` command to the
		// host server and the server removes it normally. Otherwise, the connection is
		// closed due to program failure on the guest app, we can try to revive it.

		app, exists := slicex.Find(self.apps, func(item config.App, idx int) bool {
			return item.Name == client.app
		})

		if exists {
			time.Sleep(time.Second)
			log.Printf("reviving app [%v] ...", client.app)
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
		// self.pushHandshake(messageRecord{conn: conn, msg: msg})
		self.handleHandshake(conn, msg)
	} else if msg.Cmd == "goodbye" {
		// self.pushGoodbye(messageRecord{conn: conn, msg: msg})
		self.handleGoodbye(conn, msg)
	} else if msg.Cmd == "reply" {
		// self.pushReply(messageRecord{conn: conn, msg: msg})
		self.handleReply(conn, msg)
	} else if msg.Cmd == "stop" || msg.Cmd == "reload" {
		// When the host server receives a control command, it distribute the command to the target
		// app or all apps if the app is not specified.

		if msg.App != "" {
			client, exists := self.findClient(func(item clientRecord) bool {
				return item.app == msg.App && item.app != ":cli"
			})

			if exists {
				msgId := util.RandomString()

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
				return item.app != ":cli"
			})

			if len(clients) > 0 {
				count := 0
				lock := sync.Mutex{}

				slicex.ForEach(clients, func(client clientRecord, _ int) {
					msgId := util.RandomString()

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
		// When the host server receives a `list` command, we list all the clients with names and
		// collect some information about them.
		clients := self.filterClients(func(item clientRecord) bool {
			return item.app != "" && item.app != ":cli"
		})

		if len(clients) > 0 {
			stats := []AppStat{}
			lock := sync.Mutex{}

			slicex.ForEach(clients, func(client clientRecord, _ int) {
				msgId := util.RandomString()

				client.conn.Write(EncodeMessage(ControlMessage{Cmd: "stat", MsgId: msgId}))
				self.callbacks.Set(msgId, func(reply ControlMessage) {
					lock.Lock()
					stats = append(stats, reply.Stat)

					if len(stats) == len(clients) {
						conn.Write(EncodeMessage(ControlMessage{
							Cmd:   "reply",
							Stats: stats,
							Fin:   true,
						}))
					}

					lock.Unlock()
				})
			})
		} else {
			conn.Write(EncodeMessage(ControlMessage{
				Cmd:   "reply",
				Stats: []AppStat{},
				Fin:   true,
			}))
		}
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

// NOTE: this function should only be called inside `Host.pushHandshake()`.
func (self *Host) handleHandshake(conn net.Conn, msg ControlMessage) {
	// After a guest establish the socket connection, it sends a `handshake` command indicates a
	// signing-in, we then store the client in the `hostClients` property for broadcast purposes.

	if msg.App != "" {
		self.addClient(clientRecord{conn: conn, app: msg.App})
	} else {
		self.addClient(clientRecord{conn: conn})
	}

	conn.Write(EncodeMessage(ControlMessage{Cmd: "handshake"}))

	if msg.App != "" && msg.App != ":cli" {
		cli, ok := self.findClient(func(client clientRecord) bool {
			return client.app == ":cli"
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

// NOTE: this function should only be called inside `Host.pushHandshake()`.
func (self *Host) handleGoodbye(conn net.Conn, msg ControlMessage) {
	self.removeClient(func(client clientRecord) bool {
		return client.conn == conn
	})

	if msg.Fin {
		conn.Close()
	}
}

// NOTE: this function should only be called inside `Host.pushHandshake()`.
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
			log.Printf("unable to start app [%s] (reason: %s)", app.Name, err)
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
			log.Printf("app [%s] doesn't exist in the config file", appName)
		} else if !app.Serve {
			log.Printf("app [%s] is not intended to be served", appName)
		} else {
			apps = append(apps, app)
		}
	}

	numStarted := 0

	if len(apps) != 0 {
		hasTsEntry := slicex.Some(apps, func(app config.App, _ int) bool {
			return filepath.Ext(app.Entry) == ".ts"
		})
		var err error

		if hasTsEntry {
			err = CompileTs(self.tsCfg)
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
func (self *Host) listApps(stats []AppStat) {
	var list []AppStat

	for _, app := range self.apps {
		item, exists := slicex.Find(stats, func(item AppStat, idx int) bool {
			return item.App == app.Name
		})

		if exists {
			list = append(list, item)
		} else if app.Serve {
			list = append(list, AppStat{
				App:    app.Name,
				Uri:    app.Uri,
				Pid:    -1,
				Uptime: -1,
				Memory: -1,
				Cpu:    -1,
			})
		}
	}

	tb := table.New("App", "URI", "Status", "Pid", "Uptime", "Memory", "CPU")

	for _, item := range list {
		parts := []any{item.App, item.Uri}

		if item.Pid == -1 {
			parts = append(parts, "stopped", "N/A")
		} else {
			parts = append(parts, "running", fmt.Sprint(item.Pid))
		}

		if item.Uptime == -1 {
			parts = append(parts, "N/A")
		} else {
			parts = append(parts, time.Duration(int(time.Second)*item.Uptime).String())
		}

		if item.Memory == -1 {
			parts = append(parts, "N/A")
		} else {
			parts = append(parts, fmt.Sprintf("%.2f Mb", item.Memory/1024/1024))
		}

		if item.Cpu == -1 {
			parts = append(parts, "N/A")
		} else {
			parts = append(parts, fmt.Sprintf("%.2f %%", item.Cpu))
		}

		tb.AddRow(parts...)
	}

	tb.Print()
}

// NOTE: this function runs in the CLI instead of the host server.
func (self *Host) sendCommand(cmd string, appName string) {
	guest := NewGuest(config.App{
		Name: ":cli",
		Uri:  "",
	}, func(msgId string) {})
	guest.replyChan = make(chan ControlMessage)
	err := guest.connect()

	if err != nil {
		if cmd == "list" {
			self.listApps([]AppStat{})
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
				var hasTsEntry bool

				if appName != "" {
					app, ok := slicex.Find(conf.Apps, func(app config.App, idx int) bool {
						return app.Name == appName
					})
					hasTsEntry = ok && filepath.Ext(app.Entry) == ".ts"
				} else {
					hasTsEntry = slicex.Some(conf.Apps, func(app config.App, idx int) bool {
						return filepath.Ext(app.Entry) == ".ts"
					})
				}

				if hasTsEntry {
					err = CompileTs(self.tsCfg)
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
			} else if msg.Stats != nil {
				self.listApps(msg.Stats)
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
		} else {
			guest.Leave("", "")
		}
	}
}

func IsLive() bool {
	sockFile, sockPath := GetSocketPath()

	if !util.Exists(sockFile) {
		return false
	}

	conn, err := net.DialTimeout("unix", sockPath, time.Second)

	if err != nil {
		os.Remove(sockFile) // The socket file is left by a unclean shutdown, remove it.
		return false
	} else {
		conn.Close()
		return true
	}
}

func SendCommand(cmd string, appName string) {
	config, err := config.LoadConfig()

	if err != nil {
		fmt.Println(err)
		return
	}

	host := NewHost(config)
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
			cmd = exec.Command("node", entry, app.Name)
		} else if ext == ".ts" {
			cmd = exec.Command("node", "-r", "ts-node/register", entry, app.Name)
		} else {
			cmd = exec.Command(entry, app.Name)
		}

		openFlags := os.O_CREATE | os.O_APPEND | os.O_WRONLY

		if app.Stdout != "" {
			filename := util.AbsPath(app.Stdout, false)
			util.EnsureDir(filepath.Dir(filename))
			cmd.Stdout = goext.Ok(os.OpenFile(filename, openFlags, 0644))
		} else {
			cmd.Stdout = os.Stdout
		}

		if app.Stderr != "" {
			filename := util.AbsPath(app.Stderr, false)
			util.EnsureDir(filepath.Dir(filename))
			cmd.Stderr = goext.Ok(os.OpenFile(filename, openFlags, 0644))
		} else if app.Stdout != "" {
			cmd.Stderr = goext.Ok(os.OpenFile(util.AbsPath(app.Stdout, false), openFlags, 0644))
		} else {
			cmd.Stderr = os.Stderr
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

		if outDir != "" && outFile != "" {
			entry = outFile
			env["IMPORT_ROOT"] = outDir
		}
	} else {
		entry = app.Entry
	}

	if app.Env != nil {
		env = mapx.Assign(env, app.Env)
	}

	return entry, env
}

func ResolveTsEntry(entry string, tsCfg config.TsConfig) (outDir string, outFile string) {
	if tsCfg.CompilerOptions.NoEmit {
		return "", ""
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
	}

	return outDir, outFile
}

func CompileTs(tsCfg config.TsConfig) error {
	if tsCfg.CompilerOptions.NoEmit {
		return nil
	}

	cmd := exec.Command("tsc")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil && tsCfg.CompilerOptions.NoEmitOnError {
		return err
	}

	return nil
}
