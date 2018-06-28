package common

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

// Since normal tests were tested in `source_test.go`, only test errors here.

var (
	repo        = "git@github.com:noexist/noexist.git"
	revision    = "bye"
	config      = types.GitConfig{}
	artifactURL = "https://www.baidu.com/"
)

func initTest() {
	pubkeyPath, _ := ioutil.TempFile(os.TempDir(), "pubkey-")
	pubkeyPath.WriteString("")
	defer pubkeyPath.Close()

	prikeyPath, _ := ioutil.TempFile(os.TempDir(), "prikey-")
	prikeyPath.WriteString("")
	defer prikeyPath.Close()

	config = types.GitConfig{
		PublicKey:  pubkeyPath.Name(),
		PrivateKey: prikeyPath.Name(),
		Token:      "token",
	}
}

func TestSourceCode(t *testing.T) {
	initTest()
	source := GitScm{Config: config}

	defer os.RemoveAll(config.PublicKey)
	defer os.RemoveAll(config.PrivateKey)

	path, err := ioutil.TempDir(os.TempDir(), "sourcecode-")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(path)

	// empty key
	err = source.SourceCode(repo, path, revision, false)
	assert.Error(t, err)
	//assert.Contains(t, err.Error(), "Failed to authenticate SSH session")

	// key not found
	source.Config.PrivateKey = ""
	err = source.SourceCode(repo, path, revision, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Private Key not found")

	// use https protocol
	repo = "https://github.com/hello/word.git"
	err = source.SourceCode(repo, path, revision, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Only support ssh protocol")
}

func TestGitLabArtifact(t *testing.T) {
	initTest()
	source := GitScm{Config: config}

	defer os.Remove(config.PublicKey)
	defer os.Remove(config.PrivateKey)

	err := source.Artifact(artifactURL, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zip: not a valid zip file")

	artifactURL = "http://localhost"
	err = source.Artifact(artifactURL, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}
