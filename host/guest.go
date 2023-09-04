package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ayonli/goext/slicex"
	"github.com/ayonli/goext/stringx"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/util"
	"github.com/struCoder/pidusage"
)

var startTime = time.Now()

type AppStat struct {
	App    string  `json:"app"`
	Uri    string  `json:"uri"`
	Pid    int     `json:"pid"`
	Uptime int     `json:"uptime"`
	Memory float64 `json:"memory"`
	Cpu    float64 `json:"cpu"`
}

type ControlMessage struct {
	Cmd   string    `json:"cmd"`
	App   string    `json:"app"`
	MsgId string    `json:"msgId"`
	Text  string    `json:"text"`
	Stat  AppStat   `json:"stat"`
	Stats []AppStat `json:"stats"`
	Error string    `json:"error"`

	// `Pid` shall be provided when `Cmd` is `handshake`.
	Pid int `json:"pid"`

	// `conn.Close()` will destroy the connection before the final message is flushed, causing the
	// other peer losing the connection and the message, and no EOF will be received. To guarantee
	// the final message is sent, we need a signal (`FIN`) to indicate whether this is the final
	// message, and close the connection on the receiver side.
	Fin bool `json:"fin"`
}

func EncodeMessage(msg ControlMessage) []byte {
	buf, _ := json.Marshal(msg)
	buf = append(buf, byte('\n'))

	return buf
}

func DecodeMessage(packet *[]byte, bufRead []byte, eof bool) []ControlMessage {
	*packet = append(*packet, bufRead...)
	chunks := slicex.Split(*packet, byte('\n'))

	if eof {
		// Empty the packet when reaching EOF.
		*packet = []byte{}
		// Returns all non-empty chunks, normally the last chunk is empty.
		chunks = slicex.Filter(chunks, func(chunk []byte, _ int) bool {
			return len(chunk) > 0
		})
	} else if len(chunks) > 1 {
		// The last chunk is unfinished, we store it in the packet for more data.
		*packet = chunks[len(chunks)-1]
		// All chunks (except the last one) will be processed.
		chunks = chunks[:len(chunks)-1]
	} else { // len(chunk) == 1
		// We use `\n` to delimit message packets, each packet ends with a `\n`, when len(chunks)
		// is 1, it means that the delimiter haven't been received and there is more buffers needs
		// to be received, no available chunks for consuming yet.
		return nil
	}

	messages := []ControlMessage{}

	for _, chunk := range chunks {
		var msg ControlMessage

		if err := json.Unmarshal(chunk, &msg); err != nil {
			continue
		} else {
			messages = append(messages, msg)
		}
	}

	return messages
}

func GetSocketPath() (sockFile string, sockPath string) {
	confFile := util.AbsPath("ngrpc.json", false)
	ext := filepath.Ext(confFile)
	sockFile = stringx.Slice(confFile, 0, -len(ext)) + ".sock"
	sockPath = util.AbsPath(sockFile, true)

	return sockFile, sockPath
}

type Guest struct {
	AppName           string
	AppUri            string
	conn              net.Conn
	state             int
	handleStopCommand func(msgId string)
	replyChan         chan ControlMessage
	cancelSignal      chan bool
}

func NewGuest(app config.App, onStopCommand func(msgId string)) *Guest {
	guest := &Guest{
		AppName:           app.Name,
		AppUri:            app.Uri,
		handleStopCommand: onStopCommand,
	}

	return guest
}

func (self *Guest) Join() {
	err := self.connect()

	if err != nil { // auto-reconnect in the background
		go self.reconnect()
	}
}

func (self *Guest) connect() error {
	sockFile, sockPath := GetSocketPath()

	if !util.Exists(sockFile) {
		return errors.New("host server is not running")
	}

	conn, err := net.DialTimeout("unix", sockPath, time.Second)

	if err != nil {
		// The socket file is left because a previous unclean shutdown, remove it so the filename
		// can be reused.
		os.Remove(sockFile)
		return err
	}

	msg := ControlMessage{
		Cmd: "handshake",
		App: self.AppName,
		Pid: os.Getpid(),
	}

	_, err = conn.Write(EncodeMessage(msg))

	if err != nil {
		conn.Close()
		return err
	}

	self.conn = conn
	handshake := make(chan bool)

	go func() {
		packet := []byte{}
		buf := make([]byte, 256)

		for {
			if n, err := conn.Read(buf); err != nil {
				if errors.Is(err, io.EOF) {
					self.processHostMessage(handshake, &packet, buf[:n], true)
					self.handleHostDisconnection()
					break
				} else if errors.Is(err, net.ErrClosed) {
					self.handleHostDisconnection()
					break
				} else {
					log.Println(err)
				}
			} else {
				self.processHostMessage(handshake, &packet, buf[:n], false)
			}
		}
	}()

	<-handshake

	if self.AppName != "" && self.AppName != ":cli" {
		log.Printf("app [%s] has joined the group", self.AppName)
	}

	return nil
}

func (self *Guest) Leave(reason string, replyId string) {
	if self.conn != nil {
		if replyId != "" {
			// If `replyId` is provided, that means the stop event is issued by a guest app, for
			// example, the CLI tool, in this case, we need to send feedback to acknowledge the
			// sender that the process has finished.
			self.Send(ControlMessage{Cmd: "goodbye", App: self.AppName})
			self.Send(ControlMessage{
				Cmd:   "reply",
				App:   self.AppName,
				MsgId: replyId,
				Text:  reason,
				Fin:   true,
			})
		} else {
			self.Send(ControlMessage{Cmd: "goodbye", App: self.AppName, Fin: true})
		}
	} else if self.cancelSignal != nil {
		self.cancelSignal <- true
	}

	self.state = 0
}

func (self *Guest) Send(msg ControlMessage) error {
	_, err := self.conn.Write(EncodeMessage(msg))
	return err
}

func (self *Guest) reconnect() {
	self.cancelSignal = make(chan bool)
loop:
	for self.state == 0 {
		select {
		case <-time.After(time.Second):
			err := self.connect()

			if err == nil {
				break loop
			}
		case <-self.cancelSignal:
			close(self.cancelSignal)
			break loop
		}
	}
}

func (self *Guest) handleHostDisconnection() {
	if self.state == 0 {
		return
	} else {
		self.state = 0
	}

	self.reconnect()
}

func (self *Guest) processHostMessage(
	handshake chan bool,
	packet *[]byte,
	bufRead []byte,
	eof bool,
) {
	for _, msg := range DecodeMessage(packet, bufRead, eof) {
		self.handleMessage(handshake, msg)
	}
}

func (self *Guest) handleMessage(handshake chan bool, msg ControlMessage) {
	if msg.Cmd == "handshake" {
		self.state = 1
		handshake <- true
		close(handshake)
	} else if msg.Cmd == "goodbye" {
		// self.state = 0
		self.conn.Close()

		if self.replyChan != nil {
			self.replyChan <- msg
		}
	} else if msg.Cmd == "stop" {
		self.handleStopCommand(msg.MsgId)
	} else if msg.Cmd == "reload" {
		self.Send(ControlMessage{
			Cmd:   "reply",
			MsgId: msg.MsgId,
			Text:  fmt.Sprintf("app [%v] does not support hot-reloading", self.AppName),
		})
	} else if msg.Cmd == "stat" {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		var cpuPercent float64
		sysInfo, err := pidusage.GetStat(os.Getpid())

		if err != nil {
			cpuPercent = -1
		} else {
			cpuPercent = sysInfo.CPU
		}

		self.Send(ControlMessage{
			Cmd:   "reply",
			App:   self.AppName,
			MsgId: msg.MsgId,
			Stat: AppStat{
				App:    self.AppName,
				Uri:    self.AppUri,
				Pid:    os.Getpid(),
				Uptime: int(time.Since(startTime).Round(time.Second).Seconds()),
				Memory: float64(mem.Sys + mem.NextGC),
				Cpu:    cpuPercent,
			},
		})
	} else if msg.Cmd == "reply" || msg.Cmd == "online" {
		if self.replyChan != nil {
			self.replyChan <- msg
		}
	}
}
