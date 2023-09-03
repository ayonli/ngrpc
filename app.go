package ngrpc

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"

	"github.com/ayonli/goext"
	"github.com/ayonli/goext/collections"
	"github.com/ayonli/goext/slicex"
	"github.com/ayonli/goext/structx"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/host"
	"github.com/ayonli/ngrpc/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var theApp *RpcApp

// var locks = collections.NewMap[string, *sync.Mutex]()
var serviceStore = collections.NewMap[string, any]()

type remoteInstance struct {
	app      string
	uri      string
	conn     *grpc.ClientConn
	instance any
}

type remoteService struct {
	instances []remoteInstance
	// `counter` is used to implement round-robin load-balancing algorithm.
	counter int
}

type dialer struct {
	app  *config.App
	dial func(args ...any) (*grpc.ClientConn, error)
}

// ConnectableService represents a service struct that implements the `Connect()` method.
type ConnectableService[T any] interface {
	Connect(cc grpc.ClientConnInterface) T
}

// ServableService represents a service struct that implements the `Serve()` method.
type ServableService interface {
	Serve(s grpc.ServiceRegistrar)
}

func getServiceName[T any](service ConnectableService[T]) string {
	return reflect.TypeOf(service).String()[1:]
}

// Use registers the service for use.
func Use[T any](service ConnectableService[T]) {
	serviceStore.Set(getServiceName(service), service)
}

// GetAppName retrieves the app name from the `os.Args`.
func GetAppName() string {
	if len(os.Args) >= 2 {
		return os.Args[1]
	} else {
		panic("app name is not provided")
	}
}

// Start initiates an app by the given name and loads the config file, it initiates the server
// (if served) and client connections, prepares the services ready for use.
//
// NOTE: There can only be one named app running in the same process.
func Start(appName string) (*RpcApp, error) {
	conf, err := config.LoadConfig()

	if err != nil {
		return nil, err
	} else {
		return StartWithConfig(appName, conf)
	}
}

// StartWithConfig is like `Start()` except it takes a config argument instead of loading the config
// file.
func StartWithConfig(appName string, cfg config.Config) (*RpcApp, error) {
	app, err := goext.Try(func() *RpcApp {
		if theApp != nil {
			panic(errors.New("an app is already running"))
		}

		var app *RpcApp

		if appName != "" {
			cfgApp, ok := slicex.Find(cfg.Apps, func(item config.App, _ int) bool {
				return item.Name == appName
			})

			if !ok {
				panic(fmt.Errorf("app [%v] is not configured", appName))
			}

			app = &RpcApp{App: cfgApp}

			// Initiate the server if the app is set to serve.
			if cfgApp.Serve && len(cfgApp.Services) > 0 {
				goext.Ok(0, app.initServer())
			}
		} else {
			app = &RpcApp{}
		}

		// Initiate client connections for all apps.
		goext.Ok(0, app.initClient(cfg.Apps))

		theApp = app

		if app.Name != "" {
			// TODO: could an anonymous app join the group?
			app.guest = host.NewGuest(app.App, func(msgId string) {
				app.stop(msgId, true)
			})
			app.guest.Join()
		}

		return app
	})

	if err != nil {
		if app != nil {
			app.stop("", false)
		}

		return nil, err
	} else {
		return app, nil
	}
}

// ForSnippet is used for temporary scripting usage, it runs a temporary pure-clients app that
// connects to all the services and returns a closure function which shall be called immediately
// after the snippet runs.
//
// Example:
//
//	func main() {
//		done := ngrpc.ForSnippet()
//		defer done()
//		// snippet to run
//	}
//
// See https://github.com/ayonli/ngrpc/blob/main/script/main.go for example.
func ForSnippet() func() {
	app := goext.Ok(Start(""))

	return func() {
		app.Stop()
	}
}

// GetServiceClient returns the service client (`T`).
//
// `route` is used to route traffic by the client-side load balancer.
func GetServiceClient[T any](service ConnectableService[T], route string) (T, error) {
	return goext.Try(func() T {
		if theApp == nil {
			panic("no app is running")
		}

		serviceName := getServiceName(service)
		lock, ok := theApp.locks.Get(serviceName)

		if !ok {
			panic(fmt.Errorf("service %s is not registered", serviceName))
		} else {
			lock.Lock()
		}

		record, ok := theApp.remoteServices.Get(serviceName)
		var ins T

		if !ok {
			dialers, ok := theApp.serviceDialers.Get(serviceName)

			if !ok {
				panic(fmt.Errorf("service %s is not registered", serviceName))
			}

			for _, entry := range dialers {
				// Dial the server on demand.
				conn := goext.Ok(entry.dial())

				if structx.HasMethod(service, "Connect") {
					// Calls the service's Connect() method to bind connection and gain the service
					// client.
					returns := structx.CallMethod(service, "Connect", conn)
					instance := returns[0]
					record = &remoteService{
						instances: []remoteInstance{
							{
								app:      entry.app.Name,
								uri:      entry.app.Uri,
								conn:     conn,
								instance: instance,
							},
						},
						counter: 0, // initiate the counter
					}

					// Store the instance (service client) in the unified collection for future use.
					theApp.remoteServices.Set(serviceName, record)
				}
			}
		}

		// Use only the active instances.
		instances := slicex.Filter(record.instances, func(item remoteInstance, idx int) bool {
			return item.conn.GetState() != connectivity.Shutdown
		})

		if len(instances) == 0 {
			panic(fmt.Errorf("service %s is not available", serviceName))
		}

		if route != "" { // If route is set:
			matched := false

			// First, try to match the route directly against the services' uris, if match any,
			// return it respectively.
			for _, item := range instances {
				if item.app == route || item.uri == route {
					ins = item.instance.(T)
					matched = true
					break
				}
			}

			if !matched {
				// Then, try to use the hash algorithm to retrieve a remote instance.
				idx := util.Hash(route) % len(instances)
				ins = instances[idx].instance.(T)
			}
		} else {
			// Use round-robin algorithm by default.
			idx := record.counter % len(instances)
			ins = instances[idx].instance.(T)
		}

		// Increase the service`s counter every time.
		record.counter++
		if record.counter == int(math.Pow(2, 32)) { // reset counter when it's too big
			record.counter = 0
		}

		lock.Unlock()
		return ins
	})
}

// RpcApp is used both to configure the apps and hold the app instance.
type RpcApp struct {
	config.App
	server         *grpc.Server
	clients        *collections.Map[string, *grpc.ClientConn]
	services       []ServableService
	remoteServices *collections.Map[string, *remoteService]
	serviceDialers *collections.Map[string, []dialer]
	locks          *collections.Map[string, *sync.Mutex]
	guest          *host.Guest

	// Whether this app will keep the process alive, will be set true once `WaitForExit()` is called.
	isProcessKeeper bool

	onStop func()
}

func (self *RpcApp) initServer() error {
	_, err := goext.Try(func() int {
		urlObj := goext.Ok(url.Parse(self.Uri))

		if urlObj.Scheme == "xds" {
			panic(fmt.Errorf("app [%s] cannot be served since it uses 'xds:' protocol", self.Name))
		}

		addr := config.GetAddress(urlObj)
		cred := goext.Ok(config.GetCredentials(self.App, urlObj))

		// Initiate the gRPC server
		self.server = grpc.NewServer(grpc.Creds(cred))
		self.services = []ServableService{}

		for _, serviceName := range self.Services {
			service, ok := serviceStore.Get(serviceName)

			if !ok {
				panic(fmt.Errorf("service [%s] hasn't been registered", serviceName))
			}

			// Call the service's Serve() method to initiate the service bind it to the server.
			if _service, ok := service.(ServableService); ok {
				_service.Serve(self.server)
				self.services = append(self.services, _service)

				// Dependency injection:
				//
				// If the service struct contains any field that references to another service in
				// the same server, inject that service instance into this field.
				//
				// This could be useful if we want to use that service directly without going
				// through the gRPC channel (which involve network traffic and is slower).
				//
				// However, this feature is controversial since we might not be able to guarantee if
				// the other service is running in the same process.
				value := reflect.ValueOf(_service).Elem()
				numField := value.NumField()

				for i := 0; i < numField; i++ {
					field := value.Field(i)

					if field.CanSet() {
						fieldType := field.Type()
						typeName := fieldType.String()[1:]
						target, ok := serviceStore.Get(typeName)

						if ok {
							field.Set(reflect.ValueOf(target)) // reset the field's value
						}
					}
				}
			} else {
				panic(fmt.Errorf("service [%s] doesn't implement the Serve() method", serviceName))
			}
		}

		tcpSrv := goext.Ok(net.Listen("tcp", addr))

		// Start the server in another goroutine to prevent blocking.
		go func() {
			if err := self.server.Serve(tcpSrv); err != nil {
				log.Fatal(err)
			}
		}()

		log.Printf("app [%s] started (pid: %d)", self.Name, os.Getpid())
		return 0
	})

	return err
}

// initClient initiates gRPC connections and binds client services for all the apps.
func (self *RpcApp) initClient(apps []config.App) error {
	_, err := goext.Try(func() int {
		self.clients = collections.NewMap[string, *grpc.ClientConn]()
		self.remoteServices = collections.NewMap[string, *remoteService]()
		self.serviceDialers = collections.NewMap[string, []dialer]()
		self.locks = collections.NewMap[string, *sync.Mutex]()

		slicex.ForEach(apps, func(app config.App, idx int) {
			urlObj := goext.Ok(url.Parse(app.Uri))

			var addr string

			if urlObj.Scheme == "xds" {
				addr = app.Uri // If `xds:` protocol is used, connect to it directly.
			} else {
				addr = config.GetAddress(urlObj)
			}

			cred := goext.Ok(config.GetCredentials(app, urlObj))

			// Create a dial function which will be called once the service is due to connect.
			//
			// Client connections are not established immediately, rather, they should be
			// established on demand, so that to reduce the chance of connection failure if the
			// server is not yet started.
			dial := goext.Wrap(func(args ...any) *grpc.ClientConn {
				conn, ok := self.clients.Get(app.Name)

				if ok {
					return conn
				}

				conn = goext.Ok(grpc.Dial(addr, grpc.WithTransportCredentials(cred)))
				self.clients.Set(app.Name, conn)

				return conn
			})

			slicex.ForEach(app.Services, func(serviceName string, _ int) {
				if !serviceStore.Has(serviceName) {
					panic(fmt.Errorf("service [%s] hasn't been registered", serviceName))
				}

				entries, ok := self.serviceDialers.Get(serviceName)

				if ok {
					self.serviceDialers.Set(serviceName, append(entries, dialer{
						app:  &app,
						dial: dial,
					}))
				} else {
					self.serviceDialers.Set(serviceName, []dialer{
						{
							app:  &app,
							dial: dial,
						},
					})
				}

				if !self.locks.Has(serviceName) {
					self.locks.Set(serviceName, &sync.Mutex{})
				}
			})
		})

		return 0
	})

	return err
}

// Stop closes client connections and stops the server (if served), and runs any `Stop()` method in
// the bound services.
func (self *RpcApp) Stop() {
	self.stop("", true)
}

func (self *RpcApp) stop(msgId string, graceful bool) {
	if self.clients != nil {
		self.clients.ForEach(func(conn *grpc.ClientConn, _ string) {
			conn.Close()
		})
	}

	if self.Serve && self.Services != nil && self.server != nil {
		// Call services' Stop() method before stopping the server, this is mandatory.
		for _, service := range self.services {
			if ins, ok := service.(interface{ Stop() }); ok {
				ins.Stop()
			}
		}

		self.server.Stop()
	}

	if self.onStop != nil {
		self.onStop()
	}

	if theApp == self {
		theApp = nil
	}

	var msg string

	if self.Name != "" {
		msg = fmt.Sprintf("app [%s] stopped", self.Name)
		log.Println(msg)
	} else {
		msg = "app (anonymous) stopped"
	}

	if self.guest != nil && self.guest.IsConnected() && graceful {
		self.guest.Leave(msg, msgId)

		if self.Name != "" {
			log.Printf("app [%v] has left the group", self.Name)
		}
	}

	if self.isProcessKeeper {
		os.Exit(0)
	}
}

// OnStop registers a callback to run after the app is stopped.
func (self *RpcApp) OnStop(callback func()) {
	self.onStop = callback
}

// WaitForExit blocks the main goroutine to prevent it exit prematurely unless received the
// interrupt signal from the system.
//
// This method is the default approach to keep the program running, however, we may not use it if we
// have other mechanisms to keep the program alive.
//
// This method calls the `Stop()` method internally, if we don't use this method, we need to call the
// `Stop()` method explicitly when the program is going to terminate.
func (self *RpcApp) WaitForExit() {
	self.isProcessKeeper = true
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	<-c
	self.stop("", true)
}
