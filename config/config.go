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
	"github.com/tidwall/jsonc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// App is used both to configure the apps.
type App struct {
	// The name of the app.
	Name string `json:"name"`
	// The URL of the gRPC server, supported protocols are `grpc:`, `grpcs:`, `http:`, `https:` or
	// `xds:`.
	Url string `json:"url"`
	// If this app can be served by as the gRPC server.
	Serve bool `json:"serve"`
	// The services served by this app.
	Services []string `json:"services"`
	// The certificate filename when using TLS/SSL.
	Cert string `json:"cert"`
	// The private key filename when using TLS/SSL.
	Key string `json:"key"`
	// The CA filename used to verify the other peer's certificates, when omitted, the system's root
	// CAs will be used.
	//
	// It's recommended that the gRPC application uses a self-signed certificate with a non-public
	// CA, so the client and the server can establish a private connection that no outsiders can
	// join.
	Ca     string            `json:"ca"`
	Stdout string            `json:"stdout"`
	Stderr string            `json:"stderr"`
	Entry  string            `json:"entry"`
	Env    map[string]string `json:"env"`
}

// Config is used to store configurations of the apps.
type Config struct {
	Tsconfig string `json:"tsconfig"`
	// Deprecated: use `App.Entry` instead.
	Entry      string   `json:"entry"`
	ImportRoot string   `json:"importRoot"`
	ProtoPaths []string `json:"protoPaths"`
	Apps       []App    `json:"apps"`
}

func LoadConfig() (Config, error) {
	var cfg *Config
	defaultFile := util.AbsPath("ngrpc.json", false)
	localFile := util.AbsPath("ngrpc.local.json", false)

	if util.Exists(localFile) {
		data, err := os.ReadFile(localFile)

		if err == nil {
			json.Unmarshal(jsonc.ToJSON(data), &cfg)
		}
	}

	if cfg == nil && util.Exists(defaultFile) {
		data, err := os.ReadFile(defaultFile)

		if err == nil {
			err = json.Unmarshal(jsonc.ToJSON(data), &cfg)
		}

		if err != nil {
			return *cfg, err
		}
	}

	if cfg != nil && len(cfg.Apps) > 0 {
		apps := []App{}

		for _, app := range cfg.Apps {
			if app.Entry == "" && cfg.Entry != "" {
				app.Entry = cfg.Entry
			}

			apps = append(apps, app)
		}

		cfg.Apps = apps

		return *cfg, nil
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
	var createSecure = (func(args ...any) (credentials.TransportCredentials, error) {
		return goext.Try(func() credentials.TransportCredentials {
			cert := goext.Ok(tls.LoadX509KeyPair(app.Cert, app.Key))
			var certPool *x509.CertPool

			if app.Ca != "" {
				certPool = x509.NewCertPool()
				ca := goext.Ok(os.ReadFile(app.Ca))

				if ok := certPool.AppendCertsFromPEM(ca); !ok {
					panic(fmt.Errorf("unable to create cert pool for CA: %v", app.Ca))
				}
			}

			return credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      certPool,
			})
		})
	})

	if urlObj.Scheme == "grpcs" || urlObj.Scheme == "https" {
		if app.Cert == "" {
			return nil, fmt.Errorf("missing 'Cert' config for app [%s]", app.Name)
		} else if app.Key == "" {
			return nil, fmt.Errorf("missing 'Key' config for app [%s]", app.Name)
		} else {
			return createSecure()
		}
	} else if app.Cert != "" && app.Key != "" {
		return createSecure()
	} else {
		// Create insure credentials if no certificates are set.
		return insecure.NewCredentials(), nil
	}
}
