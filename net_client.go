package yarp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
)

func NewClient(address string, opts ...Option) *Client {
	o := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	var dialer netDialer = &net.Dialer{
		Timeout: o.timeout,
	}

	if o.tlsConfig != nil {
		dialer = &tls.Dialer{
			NetDialer: dialer.(*net.Dialer),
			Config:    o.tlsConfig,
		}
	}
	c := &Client{
		address: address,
		dialer:  dialer,
		network: "tcp",
	}
	if strings.HasPrefix(address, "unix://") {
		c.network = "unix"
		c.address = strings.TrimPrefix(address, "unix://")
	}
	return c
}

type Client struct {
	address string
	dialer  netDialer
	network string
}

func (c *Client) performRequest(ctx context.Context, request Request, v interface{}) (*Response, *bufferedConn, error) {
	data, err := request.Encode()
	if err != nil {
		return nil, nil, err
	}
	encodedValue, err := Encode(v)
	if err != nil {
		return nil, nil, err
	}
	data = append(data, encodedValue...)

	conn, err := c.dialer.DialContext(ctx, c.network, c.address)
	if err != nil {
		return nil, nil, err
	}
	buf := newBufferedConn(conn)
	_, err = io.Copy(conn, bytes.NewBuffer(data))
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	// read header...
	header, err := buf.Peek(3)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	switch {
	case bytes.Equal(header, magicError):
		defer conn.Close()
		managedError := Error{}
		if err = managedError.Decode(buf); err != nil {
			return nil, nil, err
		}
		return nil, nil, managedError

	case bytes.Equal(header, magicResponse):
		response := Response{}
		if err = response.Decode(buf); err != nil {
			conn.Close()
			return nil, nil, err
		}
		return &response, &buf, err
	default:
		conn.Close()
		return nil, nil, ErrCorruptStream
	}
}

func (c *Client) DoRequest(ctx context.Context, request Request, v interface{}) (interface{}, map[string]string, error) {
	r, buf, err := c.performRequest(ctx, request, v)
	if err != nil {
		return nil, nil, err
	}
	defer buf.Close()
	if r.Stream {
		return nil, nil, ErrWantsStreamed
	}

	_, ret, err := Decode(buf)
	return &ret, r.Headers, err
}

func (c *Client) DoRequestStreamed(ctx context.Context, request Request, v interface{}) (<-chan interface{}, map[string]string, error) {
	r, buf, err := c.performRequest(ctx, request, v)
	if err != nil {
		return nil, nil, err
	}
	ch := make(chan interface{}, 10)
	go func() {
		defer buf.Close()
		defer close(ch)
		for {
			_, v, err := Decode(buf)
			if err != nil {
				break
			}
			fmt.Printf("BUG: Pushing %#v\n", v)
			ch <- v
		}
	}()
	return ch, r.Headers, nil
}
