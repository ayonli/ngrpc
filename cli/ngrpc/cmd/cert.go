package cmd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/ayonli/goext"
	"github.com/ayonli/goext/slicex"
	"github.com/ayonli/goext/stringx"
	"github.com/ayonli/ngrpc/util"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/spf13/cobra"
)

var certCmd = &cobra.Command{
	Use:   "cert <out>",
	Short: "generate a pair of self-signed certificate",
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := exec.LookPath("openssl"); err != nil {
			fmt.Println("openssl not found, please install it before generating certificates")
			return
		} else if len(args) < 1 {
			fmt.Println("the out file must be provided")
			return
		}

		caPem := cmd.Flag("ca").Value.String()
		caKey := cmd.Flag("caKey").Value.String()
		outPem := args[0]
		ext := filepath.Ext(outPem)

		if ext != ".pem" {
			fmt.Println("the out file must be suffixed with .pem")
			return
		}

		dir := filepath.Dir(outPem)
		util.EnsureDir(dir)
		outKey := stringx.Slice(outPem, 0, -len(ext)) + ".key"
		subjFields := []string{"C", "ST", "L", "O", "OU", "CN", "emailAddress"}
		subjValues := []string{}
		subjPairs := []string{}

		if util.Exists(caPem) {
			if !util.Exists(caKey) {
				fmt.Println("both ca.pem and ca.key must either exist or not exist")
				return
			}
		} else if !util.Exists(caPem) {
			_cmd := exec.Command(
				"openssl",
				"req",
				"-x509",
				"-newkey",
				"rsa:4096",
				"-nodes",
				"-days",
				"365",
				"-keyout",
				caKey,
				"--out",
				caPem)
			_cmd.Stdout = os.Stdout
			_cmd.Stderr = os.Stderr
			writer := goext.Ok(_cmd.StdinPipe())
			reader := bufio.NewReader(os.Stdin)

			go func() {
				for {
					if _cmd.ProcessState != nil && _cmd.ProcessState.Exited() {
						break
					}

					bytes, err := reader.ReadBytes('\n')

					if err != nil {
						break
					}

					subjValues = append(subjValues, string(bytes[0:len(bytes)-1]))
					writer.Write(bytes)
				}
			}()

			goext.Ok(0, _cmd.Run())
		}

		_cmd := exec.Command(
			"openssl",
			"req",
			"-newkey",
			"rsa:4096",
			"-nodes",
			"-days",
			"365",
			"-keyout",
			outKey,
			"--out",
			outPem)

		_cmd.Stdout = os.Stdout
		_cmd.Stderr = os.Stderr

		if len(subjValues) > 0 {
			for i, info := range subjValues {
				field := subjFields[i]
				subjPairs = append(subjPairs, field+"="+info)
			}

			_cmd.Args = append(_cmd.Args, "-subj", "/"+strings.Join(subjPairs, "/"))
			_cmd.Stdin = os.Stdin
		} else {
			writer := goext.Ok(_cmd.StdinPipe())
			reader := bufio.NewReader(os.Stdin)

			go func() {
				for {
					if _cmd.ProcessState != nil && _cmd.ProcessState.Exited() {
						break
					}

					bytes, err := reader.ReadBytes('\n')

					if err != nil {
						break
					}

					subjValues = append(subjValues, string(bytes[0:len(bytes)-1]))
					writer.Write(bytes)
				}
			}()
		}

		goext.Ok(0, _cmd.Run())
		hostname := subjValues[5]

		if hostname == "" {
			fmt.Println("Did you forget the Common Name in the previous steps? Try again!")
			return
		}

		ips := goext.Ok(net.LookupIP(hostname))
		var ip string

		if len(ips) > 1 {
			_ip, ok := slicex.Find(ips, func(item net.IP, idx int) bool {
				return stringx.Search(item.String(), `^\d{1,3}.\d{1,3}.\d{1,3}.\d{1,3}$`) != -1
			})

			if ok {
				ip = _ip.String()
			} else {
				ip = ips[0].String()
			}
		} else {
			ip = ips[0].String()
		}

		randId, _ := gonanoid.New()
		extCfgFile := path.Join(dir, randId+".cfg")
		extCfgContent := fmt.Sprintf("subjectAltName=DNS:%s,IP:%s\n", hostname, ip)
		os.WriteFile(extCfgFile, []byte(extCfgContent), 0644)
		defer os.Remove(extCfgFile)

		_cmd = exec.Command(
			"openssl",
			"x509",
			"-req",
			"-in",
			outPem,
			"-CA",
			caPem,
			"-CAkey",
			caKey,
			"-CAcreateserial",
			"-out",
			outPem,
			"-extfile",
			extCfgFile)
		_cmd.Stdin = os.Stdin
		_cmd.Stdout = os.Stdout
		_cmd.Stderr = os.Stderr

		goext.Ok(0, _cmd.Run())
	},
}

func init() {
	rootCmd.AddCommand(certCmd)
	certCmd.Flags().String(
		"ca",
		"certs/ca.pem",
		"use a ca.pem for signing, if doesn't exist, it will be auto-generated")
	certCmd.Flags().String(
		"caKey",
		"certs/ca.key",
		"use a ca.key for signing, if doesn't exist, it will be auto-generated")
}
