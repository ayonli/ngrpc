package config

import (
	"testing"

	"github.com/ayonli/goext"
	"github.com/stretchr/testify/assert"
)

func TestLoadTsConfig(t *testing.T) {
	tsCfg := goext.Ok(LoadTsConfig("../tsconfig.json"))
	_, err := LoadTsConfig("")

	assert.NotEqual(t, "", tsCfg.CompilerOptions.Target)
	assert.Equal(t, "open tsconfig.json: no such file or directory", err.Error())
}
