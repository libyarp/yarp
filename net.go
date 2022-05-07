package yarp

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"time"
)

type netDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// Option represents an arbitrary option to be set on a Client or Server
// instance. See WithTimeout and WithTLS.
type Option func(c *options)

type options struct {
	timeout   time.Duration
	tlsConfig *tls.Config
}

// WithTimeout determines a timeout value for a given Client or Server, and has
// different meanings depending on where it is used:
// For Server, indicates how long the server may wait for client to provide
// headers for a request, disconnecting the client in case it fails to provide
// headers under that duration.
// For Client, the value is used as a Dial timeout for the underlying
// net.Dialer.
func WithTimeout(t time.Duration) Option {
	return func(c *options) {
		c.timeout = t
	}
}

// WithTLS enables TLS support on Client and Server.
func WithTLS(config *tls.Config) Option {
	return func(c *options) {
		c.tlsConfig = config
	}
}

type bufferedConn struct {
	buf *bufio.Reader
	net.Conn
}

func newBufferedConn(c net.Conn) bufferedConn {
	return bufferedConn{bufio.NewReader(c), c}
}

func (b bufferedConn) Peek(n int) ([]byte, error) {
	return b.buf.Peek(n)
}

func (b bufferedConn) Read(p []byte) (int, error) {
	return b.buf.Read(p)
}
