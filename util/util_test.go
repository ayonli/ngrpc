package util

import (
	"os"
	"path/filepath"
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
	cwd := goext.Ok(os.Getwd())

	assert.Equal(t, filepath.Clean(cwd+"/../ngrpc.sock"), file1)
}

func TestRandomString(t *testing.T) {
	str := RandomString()

	assert.Equal(t, 8, len(str))
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
