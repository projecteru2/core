package testsuite

import (
	"bufio"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

var patBashCommand *regexp.Regexp

func init() { // nolint:gochecknoinits
	patBashCommand = regexp.MustCompile(`"\$bash\(.*?\)"[,}\]]`)
}

// MustParse .
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
			for _, r := range mustRender(request) {
				testcases <- *New(method, r, assertion)
			}
		}
	}()

	return testcases
}

func mustRender(request string) (res []string) {
	matches := patBashCommand.FindAllString(request, -1)
	if len(matches) == 0 {
		return []string{request}
	}

	for idx, match := range matches {
		switch {
		case strings.HasSuffix(match, `)",`):
			matches[idx] = strings.TrimSuffix(match, `,`)
		case strings.HasSuffix(match, `)"]`):
			matches[idx] = strings.TrimSuffix(match, `]`)
		case strings.HasSuffix(match, `)"}`):
			matches[idx] = strings.TrimSuffix(match, `}`)
		}
	}

	replacements := [][]string{}
	for _, match := range matches {
		command := strings.TrimPrefix(match, `"$bash(`)
		command = strings.TrimSuffix(command, `)"`)
		output, err := bash(command, os.Environ())
		if err != nil {
			log.Fatalf("failed to render request with command %s: %+v, %s", command, err, output)
		}
		output = strings.TrimSpace(output)
		replacements = append(replacements, strings.Split(output, "\n"))
	}

	for _, comb := range combine(replacements) {
		for idx, replace := range comb {
			res = append(res, strings.ReplaceAll(request, matches[idx], replace))
		}
	}
	return // nolint:nakedret
}
