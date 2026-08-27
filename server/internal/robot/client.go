package robot

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Client speaks the cpp-tbox raw-text terminal exposed by control_center on
// tcp_rpc (default 127.0.0.1:50000). The wire protocol is line-oriented text:
// write a command line (a terminal node path plus optional args) terminated by
// newline, then read the command's text output. The connection runs in quiet
// mode (no prompt), so responses are delimited by an idle gap rather than a
// prompt string; readIdle bounds that gap.
type Client struct {
	addr        string
	dialTimeout time.Duration
	readIdle    time.Duration

	mu   sync.Mutex
	conn net.Conn
	br   *bufio.Reader
}

func NewClient(addr string, dialTimeout, readIdle time.Duration) *Client {
	return &Client{addr: addr, dialTimeout: dialTimeout, readIdle: readIdle}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropLocked()
}

func (c *Client) dropLocked() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.br = nil
	return err
}

func (c *Client) ensureLocked() error {
	if c.conn != nil {
		return nil
	}
	conn, err := net.DialTimeout("tcp", c.addr, c.dialTimeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.addr, err)
	}
	c.conn = conn
	c.br = bufio.NewReader(conn)
	c.drainLocked()
	return nil
}

// drainLocked consumes the welcome/onBegin bytes the terminal emits on connect.
func (c *Client) drainLocked() {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.readIdle))
	buf := make([]byte, 4096)
	for {
		n, err := c.conn.Read(buf)
		if n == 0 || err != nil {
			return
		}
	}
}

func (c *Client) Exec(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	out, err := c.execOnceLocked(cmd)
	if err != nil {
		c.dropLocked()
		if out2, err2 := c.execOnceLocked(cmd); err2 == nil {
			return out2, nil
		}
		return "", err
	}
	return out, nil
}

func (c *Client) execOnceLocked(cmd string) (string, error) {
	if err := c.ensureLocked(); err != nil {
		return "", err
	}
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.dialTimeout)); err != nil {
		return "", err
	}
	if _, err := c.conn.Write([]byte(strings.TrimRight(cmd, "\r\n") + "\r\n")); err != nil {
		return "", fmt.Errorf("write %q: %w", cmd, err)
	}
	return c.readResponseLocked()
}

func (c *Client) readResponseLocked() (string, error) {
	var sb strings.Builder
	buf := make([]byte, 4096)
	got := false
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.readIdle))
		n, err := c.conn.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
			got = true
			continue
		}
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			if got {
				return sb.String(), nil
			}
			return "", fmt.Errorf("no response to command")
		}
		return "", err
	}
}
