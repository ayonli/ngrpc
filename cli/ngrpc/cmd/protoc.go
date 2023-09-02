package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc/config"
	"github.com/spf13/cobra"
)

var protocCmd = &cobra.Command{
	Use:   "protoc",
	Short: "generate golang program files from the proto files",
	Run: func(cmd *cobra.Command, args []string) {
		protoc()
	},
}

func init() {
	rootCmd.AddCommand(protocCmd)
}

func modExists(modName string) bool {
	cmd := exec.Command("go", "list", modName)
	_, err := cmd.Output()

	return err == nil
}

func ensureDeps() {
	dep1 := "google.golang.org/protobuf/cmd/protoc-gen-go"
	dep2 := "google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3"

	if !modExists(dep1) {
		goext.Ok(0, exec.Command("go", "install", dep1+"").Run())
		goext.Ok(0, exec.Command("go", "install", dep2).Run())
	}
}

func protoc() {
	_, err := exec.LookPath("protoc")

	if err != nil {
		fmt.Println(err)
		return
	}

	ensureDeps()

	conf := goext.Ok(config.LoadConfig())
	protoFileRecords := map[string][]string{}

	if len(conf.ProtoPaths) > 0 {
		for _, dir := range conf.ProtoPaths {
			filenames := scanProtoFilenames(dir)

			if len(filenames) > 0 {
				protoFileRecords[dir] = filenames
			}
		}
	} else {
		filenames := scanProtoFilenames("proto")

		if len(filenames) > 0 {
			protoFileRecords["proto"] = filenames
		}
	}

	if len(protoFileRecords) == 0 {
		fmt.Println("no proto files have been found")
		return
	}

	for root, filenames := range protoFileRecords {
		for _, filename := range filenames {
			fmt.Printf("generate code for '%s'\n", filename)
			cmd := exec.Command("protoc",
				"--proto_path="+root,
				"--go_out=./"+filepath.Join(conf.ImportRoot, "services"),
				"--go-grpc_out=./"+filepath.Join(conf.ImportRoot, "services"),
				filename)

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			cmd.Run()
		}
	}
}

func scanProtoFilenames(dir string) []string {
	filenames := []string{}

	files := goext.Ok(os.ReadDir(dir))
	protoFiles := []string{}
	subDirs := []string{}

	for _, file := range files {
		basename := file.Name()
		filename := filepath.Join(dir, basename)

		if file.IsDir() {
			subDirs = append(subDirs, filename)
		} else if strings.HasSuffix(basename, ".proto") {
			protoFiles = append(protoFiles, filename)
		}
	}

	if len(protoFiles) > 0 {
		filenames = append(filenames, protoFiles...)
	}

	if len(subDirs) > 0 {
		for _, subDir := range subDirs {
			filenames = append(filenames, scanProtoFilenames(subDir)...)
		}
	}

	return filenames
}
