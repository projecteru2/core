package testsuite

import (
	"bufio"
	"io"
	"log"
)

func MustParse(r io.Reader) <-chan Testsuite {
	testcases := make(chan Testsuite)

	go func() {
		defer close(testcases)

		scanner := bufio.NewScanner(r)
		for {
			if !scanner.Scan() {
				return
			}

			// then there must be 3 lines followed
			method := scanner.Text()
			if !scanner.Scan() {
				log.Fatalf("no request line for %s", method)
			}

			request := scanner.Text()
			if !scanner.Scan() {
				log.Fatalf("no assertion line for %s", method)
			}

			assertion := scanner.Text()
			testcases <- *New(method, request, assertion)
		}
	}()

	return testcases
}
