package cocoon

import (
	"cmp"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/log"
)

const (
	vmInfoURL = "http://cloud-hypervisor/api/v1/vm.info"

	refreshScript = `set -e
durable=$1; record=$2; console=$3; pid=$4
sed -i -e "s|\"console_socket\":\"[^\"]*\"|\"console_socket\":\"$console\"|" -e "s|\"netns_pid\":[0-9]*|\"netns_pid\":$pid|" "$durable"
mkdir -p "$(dirname "$record")"
cp -f "$durable" "$record.tmp"
mv "$record.tmp" "$record"
`
)

// vmInfo is the part of Cloud Hypervisor's vm.info the engine reads.
type vmInfo struct {
	Config struct {
		Console struct {
			Mode string `json:"mode"`
			File string `json:"file"`
		} `json:"console"`
	} `json:"config"`
}

// console is this boot's guest console: the pty of a direct-boot image, else the serial socket.
func (e *Engine) console(ctx context.Context, vm *vmRecord) (string, error) {
	dir := vm.runDir(e.cocoon.RunDir)
	serial := filepath.Join(dir, consoleSock)
	if vm.Hypervisor != cloudHypervisor {
		return serial, nil
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return e.runner.Dial(ctx, "unix", filepath.Join(dir, apiSock))
	}}}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, vmInfoURL, nil)
	if err != nil {
		return serial, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return serial, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return serial, errors.Newf("vm.info of %s: %s", vm.ID, resp.Status)
	}
	info := &vmInfo{}
	if err = json.NewDecoder(resp.Body).Decode(info); err != nil {
		return serial, err
	}
	return cmp.Or(info.Config.Console.File, serial), nil
}

// refreshRecord rewrites both copies of the meta record for this boot: the console and the VMM pid are new every boot.
func (e *Engine) refreshRecord(ctx context.Context, ID string, vm *vmRecord) error {
	console, err := e.console(ctx, vm)
	if err != nil {
		log.WithFunc("engine.cocoon.refreshRecord").WithField("ID", ID).Warnf(ctx, "the console stays the serial socket: %v", err)
	}
	_, err = e.run(ctx, sshrunner.Shell(refreshScript, durablePath(e.cocoon.Root, ID), metaPath(ID), console, strconv.Itoa(vm.PID))...)
	return err
}
