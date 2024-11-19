// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/digitalocean/go-qemu/qmp"

	"gitlab.com/kubesan/kubesan/internal/common/config"
)

const (
	QmpSockPath = "/run/qsd/qmp.sock"
)

// The NbdExport CR should be named Node-Export; but to make it easier,
// this code takes both pieces as separate items.
type ServerId struct {
	// The node to which the server should be scheduled.
	Node string

	// The export name
	Export string
}

// A QEMU Monitor Protocol (QMP) connection to a qemu-storage-daemon instance
// that is running an NBD server.
type qemuStorageDaemonMonitor struct {
	monitor qmp.Monitor
}

func newQemuStorageDaemonMonitor(path string) (*qemuStorageDaemonMonitor, error) {
	monitor, err := qmp.NewSocketMonitor("unix", path, 100*time.Millisecond)
	if err != nil {
		return nil, err
	}

	err = monitor.Connect()
	if err != nil {
		return nil, err
	}

	return &qemuStorageDaemonMonitor{monitor: monitor}, nil
}

// Closes the monitor connection and frees resources.
func (q *qemuStorageDaemonMonitor) Close() {
	_ = q.monitor.Disconnect()
}

// JSON encodes a value. Useful for avoiding escaping issues when expanding
// values into JSON snippets.
func jsonify(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// Run a QMP command ignoring error strings containing idempotencyGuard.
func (q *qemuStorageDaemonMonitor) run(cmd string, idempotencyGuard string) error {
	_, err := q.monitor.Run([]byte(cmd))
	if err != nil {
		if !strings.Contains(err.Error(), idempotencyGuard) {
			return err
		}
	}
	return nil
}

func (q *qemuStorageDaemonMonitor) BlockdevAdd(nodeName string, devicePathOnHost string) error {
	cmd := fmt.Sprintf(`
{
    "execute": "blockdev-add",
    "arguments": {
        "driver": "host_device",
        "node-name": %s,
        "cache": {
            "direct": true
        },
        "filename": %s,
        "aio": "native"
    }
}
`, jsonify(nodeName), jsonify(devicePathOnHost))

	return q.run(cmd, "Duplicate nodes with node-name")
}

func (q *qemuStorageDaemonMonitor) BlockdevDel(nodeName string) error {
	cmd := fmt.Sprintf(`
{
    "execute": "blockdev-del",
    "arguments": { "node-name": %s }
}`, jsonify(nodeName))

	return q.run(cmd, "Failed to find node with node-name")
}

func (q *qemuStorageDaemonMonitor) BlockExportAdd(id string, nodeName string, export string) error {
	cmd := fmt.Sprintf(`
{
    "execute": "block-export-add",
    "arguments": {
        "type": "nbd",
        "id": %s,
        "node-name": %s,
        "writable": true,
        "name": %s
    }
}
`, jsonify(id), jsonify(nodeName), jsonify(export))

	return q.run(cmd, " is already in use")
}

func (q *qemuStorageDaemonMonitor) BlockExportDel(id string) error {
	cmd := fmt.Sprintf(`
{
    "execute": "block-export-del",
    "arguments": {
        "id": %s,
        "mode": "hard"
    }
}
`, jsonify(id))

	return q.run(cmd, " is not found")
}

// The response to the query-block-exports QMP command
type blockExportInfo struct {
	Id           string `json:"id"`
	Type         string `json:"type"`
	NodeName     string `json:"node-name"`
	ShuttingDown bool   `json:"shutting-down"`
}

func (q *qemuStorageDaemonMonitor) QueryBlockExports() ([]blockExportInfo, error) {
	cmd := `{"execute": "query-block-exports"}`
	raw, err := q.monitor.Run([]byte(cmd))
	if err != nil {
		return nil, err
	}

	response := struct {
		Return []blockExportInfo `json:"return"`
	}{}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return nil, err
	}

	return response.Return, nil
}

// Returns the QMP node name given an NBD export name.
func nodeName(export string) string {
	return fmt.Sprintf("blockdev-%s", export)
}

// Returns the QMP block export id given an NBD export name.
func blockExportId(export string) string {
	return fmt.Sprintf("export-%s", export)
}

// Returns success only once the server is running and has the TCP port open.
func StartServer(id *ServerId, devicePathOnHost string) (string, error) {
	qsd, err := newQemuStorageDaemonMonitor(QmpSockPath)
	if err != nil {
		return "", err
	}
	defer qsd.Close()

	nodeName := nodeName(id.Export)
	err = qsd.BlockdevAdd(nodeName, devicePathOnHost)
	if err != nil {
		return "", err
	}

	blockExportId := blockExportId(id.Export)
	err = qsd.BlockExportAdd(blockExportId, nodeName, id.Export)
	if err != nil {
		return "", err
	}

	// Build NBD URI
	url := url.URL{
		Scheme: "nbd",
		Host:   config.PodIP,
		Path:   id.Export,
	}
	return url.String(), nil
}

func CheckServerHealth(id *ServerId) error {
	qsd, err := newQemuStorageDaemonMonitor(QmpSockPath)
	if err != nil {
		return err
	}
	defer qsd.Close()

	exports, err := qsd.QueryBlockExports()
	if err != nil {
		return err
	}

	blockExportId := blockExportId(id.Export)
	for i := range exports {
		if exports[i].Id == blockExportId {
			return nil // success
		}
	}

	return k8serrors.NewServiceUnavailable("NBD server unexpectedly gone")
}

func StopServer(id *ServerId) error {
	qsd, err := newQemuStorageDaemonMonitor(QmpSockPath)
	if err != nil {
		return err
	}
	defer qsd.Close()

	err = qsd.BlockExportDel(blockExportId(id.Export))
	if err != nil {
		return err
	}

	err = qsd.BlockdevDel(nodeName(id.Export))
	if err != nil {
		return err
	}
	return nil
}
