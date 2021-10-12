package main

import (
	"log"
	"os"
	"testing"

	"github.com/projecteru2/core/int/grpc_assert/pbreflect"
	"github.com/projecteru2/core/int/grpc_assert/testsuite"
)

// TestMain steps:
// 1. parse pb file
// 2. parse request_assert stream
// 3. run testcases and assertions
func TestMain(t *testing.T) {
	corepbFilename := os.Getenv("ERU_CORE_PB_FILENAME")
	service, err := pbreflect.Parse(corepbFilename)
	if err != nil {
		log.Fatalf("failed to parse pb %s: %+v", corepbFilename, err)
	}

	coreTestsuiteFilename := os.Getenv("ERU_CORE_TESTSUITE_FILENAME")
	suiteReader, err := os.Open(coreTestsuiteFilename)
	if err != nil {
		log.Fatalf("failed to open testcase file %s: %+v", coreTestsuiteFilename, err)
	}
	suites := testsuite.MustParse(suiteReader)

	for suite := range suites {
		suite.Run(t, service)
	}
}
