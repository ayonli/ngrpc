package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc/util"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// App is used both to configure the apps.
type App struct {
	// The name of the app.
	Name string `json:"name"`
	// The URI of the gRPC server, supported schemes are `grpc:`, `grpcs:`, `http:`, `https:` or
	// `xds:`.
	Uri string `json:"uri"`
	// If this app can be served by as the gRPC server.
	Serve bool `json:"serve"`
	// The services served by this app.
	Services []string `json:"services"`
	// The CA filename when using TLS/SSL.
	Ca string `json:"ca"`
	// The certificate filename when using TLS/SSL.
	Cert string `json:"cert"`
	// The private key filename when using TLS/SSL.
	Key    string            `json:"key"`
	Stdout string            `json:"stdout"`
	Stderr string            `json:"stderr"`
	Entry  string            `json:"entry"`
	Env    map[string]string `json:"env"`
}

// Config is used to store configurations of the apps.
type Config struct {
	Entry      string   `json:"entry"`
	ProtoPaths []string `json:"protoPaths"`
	Apps       []App    `json:"apps"`
}

func LoadConfig() (Config, error) {
	var config *Config
	defaultFile := util.AbsPath("ngrpc.json", false)
	localFile := util.AbsPath("ngrpc.local.json", false)

	if util.Exists(localFile) {
		data, err := os.ReadFile(localFile)

		if err == nil {
			json.Unmarshal(data, &config)
		}
	}

	if config == nil {
		if util.Exists(defaultFile) {
			data, err := os.ReadFile(defaultFile)

			if err == nil {
				json.Unmarshal(data, &config)
			} else {
				fmt.Println(err)
			}
		}
	}

	if config != nil && len(config.Apps) > 0 {
		apps := []App{}

		for _, app := range config.Apps {
			if app.Entry == "" {
				app.Entry = config.Entry
			}

			apps = append(apps, app)
		}

		config.Apps = apps

		return *config, nil
	} else {
		return Config{}, fmt.Errorf("unable to load config file: %v", defaultFile)
	}
}

func GetAddress(urlObj *url.URL) string {
	addr := urlObj.Hostname()

	if urlObj.Scheme == "grpcs" || urlObj.Scheme == "https" {
		if urlObj.Port() == "" {
			addr += ":443" // Use port 443 by default for secure protocols.
		} else {
			addr += ":" + urlObj.Port()
		}
	} else if (urlObj.Scheme == "grpc" || urlObj.Scheme == "http") && urlObj.Port() == "" {
		addr += ":80" // Use port 80 by default for insecure protocols.
	} else {
		addr += ":" + urlObj.Port()
	}

	return addr
}

func GetCredentials(app App, urlObj *url.URL) (credentials.TransportCredentials, error) {
	// Create secure (SSL/TLS) credentials, use x509 standard.
	var createSecure = goext.Wrap(func(args ...any) credentials.TransportCredentials {
		ca := goext.Ok(os.ReadFile(app.Ca))
		certPool := x509.NewCertPool()

		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			panic(fmt.Errorf("unable to create cert pool for CA: %v", app.Ca))
		}

		cert := goext.Ok(tls.LoadX509KeyPair(app.Cert, app.Key))

		return credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      certPool,
		})
	})

	if urlObj.Scheme == "grpcs" || urlObj.Scheme == "https" {
		if app.Ca == "" {
			return nil, fmt.Errorf("missing 'Ca' config for app [%s]", app.Name)
		} else if app.Cert == "" {
			return nil, fmt.Errorf("missing 'Cert' config for app [%s]", app.Name)
		} else if app.Key == "" {
			return nil, fmt.Errorf("missing 'Key' config for app [%s]", app.Name)
		} else {
			return createSecure()
		}
	} else if app.Ca != "" && app.Cert != "" && app.Key != "" {
		return createSecure()
	} else {
		// Create insure credentials if no certificates are set.
		return insecure.NewCredentials(), nil
	}
}
