package config

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc/util"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfigFile(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.json"))
	defer os.Remove("ngrpc.json")

	config := goext.Ok(LoadConfig())
	assert.True(t, len(config.Apps) > 0)
}

func TestLoadLocalConfigFile(t *testing.T) {
	goext.Ok(0, util.CopyFile("../ngrpc.json", "ngrpc.local.json"))
	defer os.Remove("ngrpc.local.json")

	config := goext.Ok(LoadConfig())
	assert.True(t, len(config.Apps) > 0)
}

func TestLoadConfigFailure(t *testing.T) {
	cwd, _ := os.Getwd()
	filename := filepath.Join(cwd, "ngrpc.json")
	config, err := LoadConfig()

	assert.Equal(t, Config{Entry: "", Apps: []App(nil)}, config)
	assert.Equal(t, "unable to load config file: "+filename, err.Error())
}

func TestGetAddress(t *testing.T) {
	urlObj1, _ := url.Parse("grpc://localhost:6000")
	urlObj2, _ := url.Parse("grpc://localhost")
	urlObj3, _ := url.Parse("grpcs://localhost")
	urlObj4, _ := url.Parse("grpcs://localhost:6000")
	addr1 := GetAddress(urlObj1)
	addr2 := GetAddress(urlObj2)
	addr3 := GetAddress(urlObj3)
	addr4 := GetAddress(urlObj4)

	assert.Equal(t, "localhost:6000", addr1)
	assert.Equal(t, "localhost:80", addr2)
	assert.Equal(t, "localhost:443", addr3)
	assert.Equal(t, "localhost:6000", addr4)
}

func TestGetCredentials(t *testing.T) {
	app1 := App{
		Name: "test-server",
		Url:  "grpc://localhost:6000",
	}
	app2 := App{
		Name: "test-server",
		Url:  "grpcs://localhost:6000",
		Ca:   "../certs/ca.pem",
		Cert: "../certs/cert.pem",
		Key:  "../certs/cert.key",
	}
	urlObj1, _ := url.Parse(app1.Url)
	urlObj2, _ := url.Parse(app2.Url)
	cred1, _ := GetCredentials(app1, urlObj1)
	cred2, _ := GetCredentials(app2, urlObj2)
	cred3, _ := GetCredentials(app2, urlObj1)

	assert.Equal(t, "insecure", cred1.Info().SecurityProtocol)
	assert.Equal(t, "tls", cred2.Info().SecurityProtocol)
	assert.Equal(t, "tls", cred3.Info().SecurityProtocol)
}

func TestGetCredentialsMissingCertFile(t *testing.T) {
	app := App{
		Name: "server-1",
		Url:  "grpcs://localhost:6000",
	}

	urlObj, _ := url.Parse(app.Url)
	_, err := GetCredentials(app, urlObj)

	assert.Equal(t, "missing 'Cert' config for app [server-1]", err.Error())
}

func TestGetCredentialsMissingKeyFile(t *testing.T) {
	app := App{
		Name: "server-1",
		Url:  "grpcs://localhost:6000",
		Cert: "../certs/cert.pem"}

	urlObj, _ := url.Parse(app.Url)
	_, err := GetCredentials(app, urlObj)

	assert.Equal(t, "missing 'Key' config for app [server-1]", err.Error())
}

func TestGetCredentialsInvalidCertFile(t *testing.T) {
	app := App{
		Name: "server-1",
		Url:  "grpcs://localhost:6000",
		Ca:   "../certs/ca.pem",
		Cert: "./certs/cert.pem",
		Key:  "./certs/cert.key",
	}

	urlObj, _ := url.Parse(app.Url)
	_, err := GetCredentials(app, urlObj)

	assert.Contains(t, err.Error(), "open ./certs/cert.pem:")
}

func TestGetCredentialsInvalidKeyFile(t *testing.T) {
	app := App{
		Name: "server-1",
		Url:  "grpcs://localhost:6000",
		Ca:   "../certs/ca.pem",
		Cert: "../certs/cert.pem",
		Key:  "./certs/cert.key",
	}

	urlObj, _ := url.Parse(app.Url)
	_, err := GetCredentials(app, urlObj)

	assert.Contains(t, err.Error(), "open ./certs/cert.key:")
}

func TestGetCredentialsBadCa(t *testing.T) {
	app := App{
		Name: "server-1",
		Url:  "grpcs://localhost:6000",
		Ca:   "../certs/ca.srl",
		Cert: "../certs/cert.pem",
		Key:  "../certs/cert.key",
	}

	urlObj, _ := url.Parse(app.Url)
	_, err := GetCredentials(app, urlObj)

	assert.Equal(t, "unable to create cert pool for CA: ../certs/ca.srl", err.Error())
}
