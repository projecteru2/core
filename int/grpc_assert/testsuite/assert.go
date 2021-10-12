package testsuite

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Equal struct {
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
}

type Appendvar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Assertion struct {
	ForEach struct {
		Equals     []Equal     `json:"equals"`
		Appendvars []Appendvar `json:"appendvars"`
	} `json:"for_each"`
	AfterCompletion struct {
		Equals     []Equal     `json:"equals"`
		Appendvars []Appendvar `json:"appendvars"`
	} `json:"after_completion"`
}

func MustNewAssertion(a string) *Assertion {
	asser := &Assertion{}
	if err := json.Unmarshal([]byte(a), asser); err != nil {
		log.Fatalf("failed to load assertion: %+v", err)
	}
	return asser
}

func (a Assertion) assertEach(t *testing.T, req, resp, err string) {
	env := []string{
		"req=" + req,
		"resp=" + resp,
		"err=" + err,
	}
	for _, equal := range a.ForEach.Equals {
		a.assert(t, equal, env)
	}
}

func (a Assertion) assertCompletion(t *testing.T, req string, resps, errs []string) {
	env := []string{
		"req=" + req,
		"resps=" + strings.Join(resps, "\n"),
		"errs=" + strings.Join(errs, "\n"),
	}
	for _, equal := range a.AfterCompletion.Equals {
		a.assert(t, equal, env)
	}
}

func (a Assertion) assert(t *testing.T, equal Equal, env []string) {
	envString := strings.Join(env, " ")
	env = append(os.Environ(), env...)
	actualOut, e := bash(equal.Actual, env)
	if e != nil {
		log.Fatalf("failed to exec bash command: %+v, %s %s", actualOut, envString, equal.Actual)
	}
	expectedOut, e := bash(equal.Expected, env)
	if e != nil {
		log.Fatalf("failed to exec bash command: %+v, %s %s", actualOut, envString, equal.Expected)
	}
	assert.EqualValues(t, actualOut, expectedOut, envString)
}

func bash(command string, env []string) (out string, err error) {
	cmd := exec.Command("/bin/bash", "-c", command)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
