package testsuite

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Equal struct {
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
}

type Assertion struct {
	ForEach struct {
		Equals     []Equal  `json:"execs"`
		RunSuccess []string `json:"run_success"`
	} `json:"for_each"`
	AfterCompletion struct {
		Equals     []Equal  `json:"equals"`
		RunSuccess []string `json:"run_success"`
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
		a.equal(t, equal, env)
	}
	for _, command := range a.ForEach.RunSuccess {
		a.run(t, command, env)
	}
}

func (a Assertion) assertCompletion(t *testing.T, req string, resps, errs []string) {
	env := []string{
		"req=" + req,
		"resps=" + strings.Join(resps, "\n"),
		"errs=" + strings.Join(errs, "\n"),
	}
	for _, equal := range a.AfterCompletion.Equals {
		a.equal(t, equal, env)
	}
	for _, command := range a.AfterCompletion.RunSuccess {
		a.run(t, command, env)
	}
}

func (a Assertion) equal(t *testing.T, equal Equal, env []string) {
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
	assert.EqualValues(t, actualOut, expectedOut, fmt.Sprintf("%s %s", envString, equal.Actual))
}

func (a Assertion) run(t *testing.T, command string, env []string) {
	output, e := bash(command, append(os.Environ(), env...))
	assert.NoError(t, e, fmt.Sprintf("output: %s, command: %s", output, command))
}
