package source

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/source/common"
	"gitlab.ricebook.net/platform/core/source/gitlab"
	"gitlab.ricebook.net/platform/core/types"
)

var (
	repo        = "git@github.com:noexist/noexist.git"
	revision    = "6"
	config      = types.GitConfig{}
	artifactURL = "http://127.0.0.1:8088/"
)

const prikeyString = `-----BEGIN RSA PRIVATE KEY-----
MIICXgIBAAKBgQC4OPXmSaaTGls9gZXO+Z2CBDeeWBPPwcOMa8wdzlws3cdUKulA
5wcblHvtN2ou1xpS9emtW5Tbo2qQFyXPL6pPYwBDq+YnkpbEGC2csMkV+JdWyTw+
4J616msS+VZ9j8Q2+calg2KL2mG504iXgCyGWu4dT9Ptplt5KWwaOsTl9wIDAQAB
AoGAZqxocHbv/eCcpYUJp5d7b7FGBlx0fkAx6ptR4fLXcLISnBhmdCPO1FJHV4ih
B4YfR8mC+XmnV1qW08Py8KxSMIVBjqFxunv2+p9RhwnP0K8Pi3OuQO1EsUMeSRwG
w2AXAgDdlBuLQB2j6ixsJsKfCdA7El7qu8WDNwozWwwacTECQQDxZfdrgOS+xDNn
Nm1bEcQySOOJH2x1d+sR8pLQT5nHs7Q9DXvBdJvMtcgPkWltmtoueap+GjrmE4uZ
qmUmDHsFAkEAw12hnE4iADSVuLAn+vYdLNebY0GdB5MSumuouXaVrzyqNgwDYk6h
2BGgxUC8LPcZePhAAZZMqRpwBpRaLotFywJBAI4glO4ss4FGD2XDe9tUuIlKtPz1
DWyUMEke4yXW2BnmSkZv+99JAroihSn1WXd45uDaLXGVi/wOofDVjDw8uOkCQQCK
xwY4HCB2+OOqMCgWU6Hh6r6MwV0ktkrFdhiCtkQaGQPoJJx6xtScwdjshdGmN1k2
31HITtXiAc+2PMfa7EAFAkEAtyqskXhSzLcOS0WWQnfhy0m5oA7lnlFYi7SaWAHo
+VwICwafZ3oMc+pAqHzIDb9h1wnX5O++YafDbGeiT+7QvA==
-----END RSA PRIVATE KEY-----`

const pubkeyString = `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQC4OPXmSaaTGls9gZXO+Z2CBDeeWBPPwcOMa8wdzlws3cdUKulA5wcblHvtN2ou1xpS9emtW5Tbo2qQFyXPL6pPYwBDq+YnkpbEGC2csMkV+JdWyTw+4J616msS+VZ9j8Q2+calg2KL2mG504iXgCyGWu4dT9Ptplt5KWwaOsTl9w== mr@rabbit`

func initTest() {
	pubkeyPath, _ := ioutil.TempFile(os.TempDir(), "pubkey-")
	pubkeyPath.WriteString(pubkeyString)
	defer pubkeyPath.Close()

	prikeyPath, _ := ioutil.TempFile(os.TempDir(), "prikey-")
	prikeyPath.WriteString(prikeyString)
	defer prikeyPath.Close()

	config = types.GitConfig{
		PublicKey:  pubkeyPath.Name(),
		PrivateKey: prikeyPath.Name(),
		Token:      "x",
	}
}

func TestGitSourceCode(t *testing.T) {
	initTest()
	source := common.GitScm{Config: config}

	defer os.RemoveAll(config.PublicKey)
	defer os.RemoveAll(config.PrivateKey)

	path, err := ioutil.TempDir(os.TempDir(), "sourcecode-")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(path)

	err = source.SourceCode(repo, path, revision)
	assert.Error(t, err)
	assert.EqualError(t, err, "Failed to authenticate SSH session: Waiting for USERAUTH response")
}

func simpleHTTPServer(file *os.File) {
	zipFile, _ := os.Open(fmt.Sprintf("%s.zip", file.Name()))
	defer zipFile.Close()

	if err := common.ZipFiles(zipFile.Name(), []string{file.Name()}); err != nil {
		log.Fatal(err)
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, zipFile.Name())
	})

	err := http.ListenAndServe(":8088", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func TestGitLabArtifact(t *testing.T) {
	file, _ := ioutil.TempFile(os.TempDir(), "file-")
	defer file.Close()
	defer os.Remove(file.Name())
	file.WriteString(prikeyString)

	zipFile, _ := os.Create(fmt.Sprintf("%s.zip", file.Name()))
	defer zipFile.Close()
	defer os.Remove(zipFile.Name())

	go simpleHTTPServer(file)
	// give 1s to httpServer zip the file and start up
	time.Sleep(1 * time.Second)

	initTest()
	source := gitlab.New(config)

	defer os.Remove(config.PublicKey)
	defer os.Remove(config.PrivateKey)

	path, err := ioutil.TempDir(os.TempDir(), "sourcecode-")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(path)

	err = source.Artifact(artifactURL, path)
	assert.NoError(t, err)

	fileName := strings.TrimPrefix(file.Name(), os.TempDir())
	if strings.HasPrefix(fileName, "/") {
		fileName = strings.TrimPrefix(fileName, "/")
	}
	downloadFilePath := fmt.Sprintf("%s/%s", path, fileName)
	fmt.Println(downloadFilePath)

	downloadFile, err := os.Open(downloadFilePath)
	assert.NoError(t, err)
	artifactString, _ := ioutil.ReadAll(downloadFile)
	// diff the artifact and the origin
	assert.Equal(t, string(artifactString), prikeyString)
}
