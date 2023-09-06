package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/pm"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <filename> [args...]",
	Short: "run a script program",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("filename must be provided")
			return
		}

		filename := args[0]
		ext := filepath.Ext(filename)
		env := map[string]string{}
		var script *exec.Cmd

		if ext == ".go" {
			script = exec.Command("go", "run", filename)
		} else if ext == ".ts" {
			cfg := goext.Ok(config.LoadConfig())
			tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
			outDir, outFile := pm.ResolveTsEntry(filename, tsCfg)
			goext.Ok(0, pm.CompileTs(tsCfg, outDir))
			script = exec.Command("node", outFile)
			env["IMPORT_ROOT"] = outDir
		} else if ext == ".js" {
			script = exec.Command("node", filename)
		}

		if len(args) > 1 {
			script.Args = append(script.Args, args[1:]...)
		}

		script.Stdin = os.Stdin
		script.Stdout = os.Stdout
		script.Stderr = os.Stderr

		if len(env) > 0 {
			script.Env = os.Environ()

			for key, value := range env {
				script.Env = append(script.Env, key+"="+value)
			}
		}

		script.Run()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
