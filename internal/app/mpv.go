package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// mpvPollTimeout bounds one request to the player. mpv answers in
// milliseconds; a timeout means the player is gone.
const mpvPollTimeout = 2 * time.Second

// mpvSocketPath returns the address of the IPC socket of one player. On
// Windows mpv uses a named pipe, everywhere else a socket next to the
// temporary directory of the run.
func mpvSocketPath(dir string) string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\orangeflix-mpv-` + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return filepath.Join(dir, "ipc.sock")
}

// openMPVSocket connects to the socket of the player.
func openMPVSocket(path string) (io.ReadWriteCloser, error) {
	if runtime.GOOS == "windows" {
		return os.OpenFile(path, os.O_RDWR, 0)
	}
	return net.Dial("unix", path)
}

// errPlayerExited tells that the player ended before its socket was there.
var errPlayerExited = errors.New("der Player wurde beendet")

// dialMPV waits until the player created its socket. The player needs a
// moment to start; the wait ends when it exits or the deadline passes.
func dialMPV(ctx context.Context, path string, exited <-chan error) (*mpvIPC, error) {
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for {
		conn, err := openMPVSocket(path)
		if err == nil {
			return newMPVIPC(conn), nil
		}
		last = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-exited:
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errPlayerExited, err)
			}
			return nil, errPlayerExited
		default:
		}
		if time.Now().After(deadline) {
			return nil, last
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// mpvAnswer is the answer of one request.
type mpvAnswer struct {
	Data  json.RawMessage
	Error string
}

// mpvIPC speaks the JSON IPC of mpv. One reader goroutine dispatches the
// answers to the requests and keeps the events up to date.
type mpvIPC struct {
	conn io.ReadWriteCloser

	writeMu sync.Mutex
	nextID  atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan mpvAnswer

	eof       atomic.Bool
	closed    chan struct{}
	closeOnce sync.Once
}

// newMPVIPC starts the reader of the connection.
func newMPVIPC(conn io.ReadWriteCloser) *mpvIPC {
	client := &mpvIPC{
		conn:    conn,
		pending: map[int64]chan mpvAnswer{},
		closed:  make(chan struct{}),
	}
	go client.read()
	return client
}

// read reads the messages until the connection breaks.
func (c *mpvIPC) read() {
	decoder := json.NewDecoder(c.conn)
	for {
		var message struct {
			Event     string          `json:"event"`
			Name      string          `json:"name"`
			Reason    string          `json:"reason"`
			Data      json.RawMessage `json:"data"`
			RequestID int64           `json:"request_id"`
			Error     string          `json:"error"`
		}
		if err := decoder.Decode(&message); err != nil {
			c.Close()
			return
		}
		if message.Event == "property-change" && message.Name == "eof-reached" {
			var reached bool
			if json.Unmarshal(message.Data, &reached) == nil {
				c.eof.Store(reached)
			}
		}
		// mpv meldet das Ende der Datei auch dann, wenn die Eigenschaft
		// nicht mehr abgefragt werden kann.
		if message.Event == "end-file" && message.Reason == "eof" {
			c.eof.Store(true)
		}
		if message.RequestID == 0 {
			continue
		}
		c.pendingMu.Lock()
		answer := c.pending[message.RequestID]
		delete(c.pending, message.RequestID)
		c.pendingMu.Unlock()
		if answer != nil {
			answer <- mpvAnswer{Data: message.Data, Error: message.Error}
		}
	}
}

// send writes one command without waiting for the answer.
func (c *mpvIPC) send(command ...any) error {
	payload, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.conn.Write(payload)
	return err
}

// command sends a command and waits for its answer.
func (c *mpvIPC) command(command ...any) (mpvAnswer, error) {
	id := c.nextID.Add(1)
	answer := make(chan mpvAnswer, 1)
	c.pendingMu.Lock()
	c.pending[id] = answer
	c.pendingMu.Unlock()

	payload, err := json.Marshal(map[string]any{"command": command, "request_id": id})
	if err != nil {
		c.forget(id)
		return mpvAnswer{}, err
	}
	payload = append(payload, '\n')
	c.writeMu.Lock()
	_, err = c.conn.Write(payload)
	c.writeMu.Unlock()
	if err != nil {
		c.forget(id)
		return mpvAnswer{}, err
	}

	timer := time.NewTimer(mpvPollTimeout)
	defer timer.Stop()
	select {
	case result := <-answer:
		return result, nil
	case <-timer.C:
		c.forget(id)
		return mpvAnswer{}, errors.New("der Player antwortet nicht")
	case <-c.closed:
		return mpvAnswer{}, io.EOF
	}
}

// forget drops a pending request.
func (c *mpvIPC) forget(id int64) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

// observe subscribes to a property.
func (c *mpvIPC) observe(name string) error {
	return c.send("observe_property", c.nextID.Add(1), name)
}

// property returns the current value of a property.
func (c *mpvIPC) property(name string) (json.RawMessage, error) {
	answer, err := c.command("get_property", name)
	if err != nil {
		return nil, err
	}
	if answer.Error != "" && answer.Error != "success" {
		return nil, fmt.Errorf("die Eigenschaft %s ist nicht verfügbar: %s", name, answer.Error)
	}
	return answer.Data, nil
}

// number returns a numeric property. The second result tells whether the
// value was there.
func (c *mpvIPC) number(name string) (float64, bool, error) {
	data, err := c.property(name)
	if err != nil {
		return 0, false, err
	}
	if len(data) == 0 || string(data) == "null" {
		return 0, false, nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err != nil {
		return 0, false, err
	}
	return value, true, nil
}

// reachedEnd reports whether the player reached the end of the file.
func (c *mpvIPC) reachedEnd() bool {
	return c.eof.Load()
}

// Close releases the connection.
func (c *mpvIPC) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
	})
}
