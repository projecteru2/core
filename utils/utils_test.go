package utils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestRandomString(t *testing.T) {
	s1 := RandomString(10)
	assert.Equal(t, 10, len(s1))
	s2 := RandomString(10)
	assert.Equal(t, 10, len(s2))
	assert.NotEqual(t, s1, s2, fmt.Sprintf("s1: %s, s2: %s", s1, s2))
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

	_, err = GetGitRepoName("file://github.com/projecteru2/core.git")
	assert.Error(t, err)

	_, err = GetGitRepoName("https://github.com/projecteru2/core.git")
	assert.NoError(t, err)

	r1, err := GetGitRepoName("git@github.com:projecteru2/core.git")
	assert.NoError(t, err)
	assert.Equal(t, r1, "core")
}

func TestGetTag(t *testing.T) {
	v := GetTag("xx")
	assert.Equal(t, v, DefaultVersion)
	v = GetTag("xx:1:2")
	assert.Equal(t, v, "2")
	v = GetTag("xx:2")
	assert.Equal(t, v, "2")
}

func TestNormalizeImageName(t *testing.T) {
	i := NormalizeImageName("image")
	assert.Equal(t, i, "image:latest")
	i = NormalizeImageName("image:1")
	assert.Equal(t, i, "image:1")
}

func TestMakeCommandLine(t *testing.T) {
	r1 := MakeCommandLineArgs("/bin/bash -l -c 'echo \"foo bar bah bin\"'")
	assert.Equal(t, r1, []string{"/bin/bash", "-l", "-c", "echo \"foo bar bah bin\""})
	r2 := MakeCommandLineArgs(" test -a   -b   -d")
	assert.Equal(t, r2, []string{"test", "-a", "-b", "-d"})
}

func TestMakeWorkloadName(t *testing.T) {
	r1 := MakeWorkloadName("test_appname", "web", "whatever")
	assert.Equal(t, r1, "test_appname_web_whatever")
	appname, entrypoint, ident, err := ParseWorkloadName("test_appname_web_whatever")
	assert.Equal(t, appname, "test_appname")
	assert.Equal(t, entrypoint, "web")
	assert.Equal(t, ident, "whatever")
	assert.Equal(t, err, nil)
}

func TestParseWorkloadName(t *testing.T) {
	appname := "test_bad"
	p1, p2, p3, err := ParseWorkloadName(appname)
	assert.Error(t, err)
	assert.Equal(t, p1, "")
	assert.Equal(t, p2, "")
	assert.Equal(t, p3, "")
	appname = "test_good_name_1"
	p1, p2, p3, err = ParseWorkloadName(appname)
	assert.NoError(t, err)
	assert.Equal(t, p1, "test_good")
	assert.Equal(t, p2, "name")
	assert.Equal(t, p3, "1")
}

func TestPublishInfo(t *testing.T) {
	ports := []string{"123", "233"}
	n1 := map[string]string{
		"n1":   "233.233.233.233",
		"host": "127.0.0.1",
	}
	r := MakePublishInfo(n1, ports)
	assert.Equal(t, len(r), 2)
	assert.Equal(t, len(r["n1"]), 2)
	assert.Equal(t, r["n1"][0], "233.233.233.233:123")
	assert.Equal(t, r["n1"][1], "233.233.233.233:233")
	assert.Equal(t, len(r["host"]), 2)
	assert.Equal(t, r["host"][0], "127.0.0.1:123")
	assert.Equal(t, r["host"][1], "127.0.0.1:233")

	e := EncodePublishInfo(r)
	assert.Equal(t, len(e), 2)
	assert.Equal(t, e["n1"], "233.233.233.233:123,233.233.233.233:233")
	assert.Equal(t, e["host"], "127.0.0.1:123,127.0.0.1:233")

	r2 := DecodePublishInfo(e)
	assert.Equal(t, len(r2), 2)
	assert.Equal(t, len(r2["n1"]), 2)
	assert.Equal(t, len(r2["host"]), 2)
	assert.Equal(t, r2["n1"][0], "233.233.233.233:123")
	assert.Equal(t, r2["n1"][1], "233.233.233.233:233")
	assert.Equal(t, r2["host"][0], "127.0.0.1:123")
	assert.Equal(t, r2["host"][1], "127.0.0.1:233")
}

func TestMetaInLabel(t *testing.T) {
	meta := &types.LabelMeta{
		Publish: []string{"1", "2"},
	}
	r := EncodeMetaInLabel(meta)
	assert.NotEmpty(t, r)

	labels := map[string]string{
		cluster.LabelMeta: "{\"Publish\":[\"5001\"],\"HealthCheck\":{\"TCPPorts\":[\"5001\"],\"HTTPPort\":\"\",\"HTTPURL\":\"\",\"HTTPCode\":0}}",
	}
	meta2 := DecodeMetaInLabel(labels)
	assert.Equal(t, meta2.Publish[0], "5001")
	meta3 := DecodeMetaInLabel(map[string]string{cluster.LabelMeta: ""})
	assert.Nil(t, meta3.HealthCheck)
}

func TestShortID(t *testing.T) {
	r1 := ShortID("1234")
	assert.Equal(t, r1, "1234")
	r2 := ShortID("12345678")
	assert.Equal(t, r2, "1234567")
}

func TestFilterWorkload(t *testing.T) {
	e := map[string]string{"a": "b"}
	l := map[string]string{"a": "b"}
	assert.True(t, FilterWorkload(e, l))
	l["c"] = "d"
	assert.False(t, FilterWorkload(e, l))
}

func TestCleanStatsdMetrics(t *testing.T) {
	k := "a.b.c"
	assert.Equal(t, CleanStatsdMetrics(k), "a-b-c")
}

func TestTempFile(t *testing.T) {
	buff := bytes.NewBufferString("test")
	rc := ioutil.NopCloser(buff)
	fname, err := TempFile(rc)
	assert.NoError(t, err)
	f, err := os.Open(fname)
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, string(b), "test")
	os.Remove(fname)
}

func TestRound(t *testing.T) {
	f := func(f float64) string {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	a := 0.0199999998
	assert.Equal(t, f(Round(a)), "0.02")
	a = 0.1999998
	assert.Equal(t, f(Round(a)), "0.2")
	a = 1.999998
	assert.Equal(t, f(Round(a)), "1.999998")
	a = 19.99998
	assert.Equal(t, f(Round(a)), "19.99998")
}

func TestMergeHookOutputs(t *testing.T) {
	test := []*bytes.Buffer{bytes.NewBufferString("a"), bytes.NewBufferString("b")}
	r := MergeHookOutputs(test)
	assert.NotEmpty(t, r)
	assert.Equal(t, string(r), "ab")
}

func TestMin(t *testing.T) {
	var a int
	var b int
	a = 1
	b = 2
	assert.Equal(t, 1, Min(a, b))
	assert.Equal(t, 1, Min(b, a))
}

func TestMin64(t *testing.T) {
	var a int64
	var b int64
	a = 1
	b = 2
	assert.Equal(t, int64(1), Min64(a, b))
	assert.Equal(t, int64(1), Min64(b, a))
}

func TestMax(t *testing.T) {
	assert.Equal(t, 3, Max(1, 2, 3))
	assert.Equal(t, 4, Max(1, 4, 3))
}

func TestEnsureReaderClosed(t *testing.T) {
	EnsureReaderClosed(nil)
	s := ioutil.NopCloser(bytes.NewBuffer([]byte{10, 10, 10}))
	EnsureReaderClosed(s)
}

func TestRange(t *testing.T) {
	res := Range(10)
	assert.Equal(t, 10, len(res))
	for i := 0; i < 10; i++ {
		assert.Equal(t, i, res[i])
	}
}
