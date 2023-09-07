package util

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/ayonli/goext"
	"github.com/stretchr/testify/assert"
)

func TestExists(t *testing.T) {
	ok1 := Exists("../ngrpc.json")
	ok2 := Exists("../ngrpc.local.json")

	assert.True(t, ok1)
	assert.False(t, ok2)
}

func TestAbsPath(t *testing.T) {
	file1 := AbsPath("../ngrpc.sock", false)
	file2 := AbsPath("/usr/local/bin", false)
	cwd := goext.Ok(os.Getwd())

	assert.Equal(t, filepath.Clean(cwd+"/../ngrpc.sock"), file1)
	assert.Equal(t, "/usr/local/bin", file2)

	if runtime.GOOS == "windows" {
		filename := "C:\\Program Files\\go\\bin"
		file3 := AbsPath(filename, false)
		assert.Equal(t, filename, file3)

		file4 := AbsPath(filename, true)
		assert.Equal(t, "\\\\.\\pipe\\"+filename, file4)
	}
}

func TestHash(t *testing.T) {
	hash := Hash("hello, world!")

	assert.Equal(t, 10, len(strconv.Itoa(hash)))
}

func TestEnsureDir(t *testing.T) {
	assert.False(t, Exists("test/foo/bar"))

	goext.Ok(0, EnsureDir("test/foo/bar"))

	assert.True(t, Exists("test/foo/bar"))

	goext.Ok(0, os.Remove("test/foo/bar"))
	goext.Ok(0, os.Remove("test/foo"))
	goext.Ok(0, os.Remove("test"))
}

func TestCopyFile(t *testing.T) {
	goext.Ok(0, CopyFile("../ngrpc.json", "ngrpc.json"))
	defer os.Remove("ngrpc.json")

	srcContents := goext.Ok(os.ReadFile("../ngrpc.json"))
	dstContents := goext.Ok(os.ReadFile("ngrpc.json"))

	assert.Equal(t, srcContents, dstContents)
}

func TestGetPidStat(t *testing.T) {
	stat := goext.Ok(GetPidStat(os.Getpid()))
	assert.True(t, stat.Memory > 0)
	assert.True(t, stat.CPU >= 0.00)
}
