package systemd

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

type serviceStatus struct {
	SubState    string
	ActiveState string
	Environment string
	Description string
	User        string
}

func newServiceStatus(buf *bytes.Buffer) *serviceStatus {
	status := map[string]string{}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		text := scanner.Text()
		parts := strings.SplitN(text, "=", 2)
		status[parts[0]] = parts[1]
	}
	return &serviceStatus{
		SubState:    status["SubState"],
		ActiveState: status["ActiveState"],
		Environment: status["Environment"],
		Description: status["Description"],
		User:        status["User"],
	}
}

func (s *serviceStatus) running() bool {
	return s.SubState == "running" && s.ActiveState == "active"
}

func (s *serviceStatus) env() ([]string, error) {
	reader := csv.NewReader(strings.NewReader(s.Environment))
	reader.Comma = ' '
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errors.Wrap(err, s.Environment)
	}
	return records[0], nil
}

func (s *serviceStatus) labels() (map[string]string, error) {
	desc := &unitDesciption{}
	description := strings.ReplaceAll(s.Description, "\\x5c", "\\")
	if err := json.Unmarshal([]byte(description), desc); err != nil {
		return nil, errors.Wrap(err, s.Description)
	}
	return desc.Labels, nil
}
