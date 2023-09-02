package config

import (
	"encoding/json"
	"os"

	"github.com/tidwall/jsonc"
)

type CompilerOptions struct {
	Module        string `json:"json"`
	Target        string `json:"target"`
	OutDir        string `json:"outDir"`
	NoEmit        bool   `json:"noEmit"`
	NoEmitOnError bool   `json:"noEmitOnError"`
}

type TsConfig struct {
	CompilerOptions CompilerOptions `json:"compilerOptions"`
	Includes        []string        `json:"includes"`
}

func LoadTsConfig(filename string) (TsConfig, error) {
	var tsConfig TsConfig

	fileContents, err := os.ReadFile("tsconfig.json")

	if err != nil {
		return tsConfig, err
	}

	err = json.Unmarshal(jsonc.ToJSON(fileContents), &tsConfig)

	return tsConfig, err
}
