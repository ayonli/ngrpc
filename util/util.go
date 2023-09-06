package util

import (
	"errors"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ayonli/goext/stringx"
)

func Exists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	} else if errors.Is(err, os.ErrNotExist) {
		return false
	} else {
		panic(err)
	}
}

func AbsPath(filename string, pipePrefix bool) string {
	isAbs := false

	if stringx.Search(filename, `^/|^[a-zA-Z]:[\\/]`) == -1 {
		cwd, _ := os.Getwd()
		filename = filepath.Join(cwd, filename)
	} else {
		isAbs = true
	}

	if !isAbs {
		if filepath.Separator == '/' {
			filename = strings.ReplaceAll(filename, "\\", "/")
		} else if filepath.Separator == '\\' {
			filename = strings.ReplaceAll(filename, "/", "\\")
		}
	}

	if pipePrefix && runtime.GOOS == "windows" && stringx.Search(filename, "^\\\\[.?]\\pipe\\") == -1 {
		return "\\\\.\\pipe\\" + filename
	} else {
		return filename
	}
}

func Hash(str string) int {
	hash := fnv.New32()
	hash.Write([]byte(str))
	return int(hash.Sum32())
}

func EnsureDir(dirname string) error {
	if err := os.MkdirAll(dirname, 0755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		} else {
			return err
		}
	}

	return nil
}

func CopyFile(src string, dst string) error {
	in, err := os.Open(src)

	if err != nil {
		return err
	}

	out, err := os.Create(dst)

	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)

	if err == nil { // must close the files for Windows
		in.Close()
		out.Close()
	}

	return err
}
