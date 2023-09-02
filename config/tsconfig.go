package config

import (
	"encoding/json"
	"os"

	"github.com/tidwall/jsonc"
)

type CompilerOptions struct {
	Module        string `json:"json"`
	Target        string `json:"target"`
	RootDir       string `json:"rootDir"`
	OutDir        string `json:"outDir"`
	NoEmit        bool   `json:"noEmit"`
	NoEmitOnError bool   `json:"noEmitOnError"`
}

type TsConfig struct {
	CompilerOptions CompilerOptions `json:"compilerOptions"`
	Includes        []string        `json:"includes"`
}

func LoadTsConfig(filename string) (TsConfig, error) {
	var tsCfg TsConfig

	if filename == "" {
		filename = "tsconfig.json"
	}

	fileContents, err := os.ReadFile(filename)

	if err != nil {
		return tsCfg, err
	}

	err = json.Unmarshal(jsonc.ToJSON(fileContents), &tsCfg)

	return tsCfg, err
}
