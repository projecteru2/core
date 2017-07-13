package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomString(t *testing.T) {
	s1 := RandomString(10)
	assert.Equal(t, 10, len(s1))
	s2 := RandomString(10)
	assert.Equal(t, 10, len(s2))
	assert.NotEqual(t, s1, s2, fmt.Sprintf("s1: %s, s2: %s", s1, s2))
}

func TestTruncateID(t *testing.T) {
	r1 := TruncateID("1234")
	assert.Equal(t, r1, "1234")
	r2 := TruncateID("12345678")
	assert.Equal(t, r2, "1234567")
}

func TestTail(t *testing.T) {
	r1 := Tail("")
	assert.Equal(t, r1, "")
	r2 := Tail("/")
	assert.Equal(t, r2, "")
	r3 := Tail("a/b")
	assert.Equal(t, r3, "b")
	r4 := Tail("a/b/c")
	assert.Equal(t, r4, "c")
}

func TestGetGitRepoName(t *testing.T) {
	_, err := GetGitRepoName("xxx")
	assert.Error(t, err)

	_, err = GetGitRepoName("http://gitlab.ricebook.net/platform/core.git")
	assert.Error(t, err)

	r1, err := GetGitRepoName("git@gitlab.ricebook.net:platform/core.git")
	assert.NoError(t, err)
	assert.Equal(t, r1, "core")
}

func TestMakeCommandLine(t *testing.T) {
	r1 := MakeCommandLineArgs("/bin/bash -l -c 'echo \"foo bar bah bin\"'")
	assert.Equal(t, r1, []string{"/bin/bash", "-l", "-c", "echo \"foo bar bah bin\""})
	r2 := MakeCommandLineArgs(" test -a   -b   -d")
	assert.Equal(t, r2, []string{"test", "-a", "-b", "-d"})
}

func TestMakeContainerName(t *testing.T) {
	r1 := MakeContainerName("test_appname", "web", "whatever")
	assert.Equal(t, r1, "test_appname_web_whatever")
	appname, entrypoint, ident, err := ParseContainerName("test_appname_web_whatever")
	assert.Equal(t, appname, "test_appname")
	assert.Equal(t, entrypoint, "web")
	assert.Equal(t, ident, "whatever")
	assert.Equal(t, err, nil)
}
