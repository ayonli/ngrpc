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
	assert.Contains(t, err.Error(), "open tsconfig.json:")
}
