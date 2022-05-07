package yarp

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

type RequestType struct {
	*Structure
}

func (RequestType) YarpID() uint64         { return 0x0 }
func (RequestType) YarpPackage() string    { return "io.libyarp" }
func (RequestType) YarpStructName() string { return "RequestType" }

type ResponseType struct {
	*Structure
}

func (ResponseType) YarpID() uint64         { return 0x0 }
func (ResponseType) YarpPackage() string    { return "io.libyarp" }
func (ResponseType) YarpStructName() string { return "RequestType" }

type ResponseTypeStreamer struct {
	h  Header
	ch chan<- *ResponseType
}

func (i ResponseTypeStreamer) Headers() Header      { return i.h }
func (i ResponseTypeStreamer) Push(v *ResponseType) { i.ch <- v }

func TestServerRegisterReflect(t *testing.T) {
	t.Run("request, response, no stream", func(t *testing.T) {
		handler := func(ctx context.Context, headers Header, req *RequestType) (Header, *ResponseType, error) {
			return nil, nil, nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		assert.False(t, hnd.usesStreamer)
		assert.Zero(t, hnd.streamerType)
		assert.Equal(t, reflect.TypeOf(&RequestType{}), hnd.inType)
		assert.Equal(t, reflect.TypeOf(&ResponseType{}), hnd.outType)
	})

	t.Run("void request, response, no stream", func(t *testing.T) {
		handler := func(ctx context.Context, headers Header) (Header, *ResponseType, error) {
			return nil, nil, nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		assert.False(t, hnd.usesStreamer)
		assert.Zero(t, hnd.streamerType)
		assert.Zero(t, hnd.inType)
		assert.Equal(t, reflect.TypeOf(&ResponseType{}), hnd.outType)
	})

	t.Run("void request, void response, no stream", func(t *testing.T) {
		handler := func(ctx context.Context, headers Header) (Header, error) {
			return nil, nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		assert.False(t, hnd.usesStreamer)
		assert.Zero(t, hnd.streamerType)
		assert.Zero(t, hnd.inType)
		assert.Zero(t, hnd.outType)
	})

	t.Run("request, response, stream", func(t *testing.T) {
		handler := func(ctx context.Context, headers Header, req *RequestType, res *ResponseTypeStreamer) error {
			return nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		assert.True(t, hnd.usesStreamer)
		assert.Equal(t, reflect.TypeOf(ResponseTypeStreamer{}), hnd.streamerType)
		assert.Equal(t, hnd.inType, reflect.TypeOf(&RequestType{}))
		assert.Zero(t, hnd.outType)
	})

	t.Run("void request, response, stream", func(t *testing.T) {
		handler := func(ctx context.Context, headers Header, res *ResponseTypeStreamer) error {
			return nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		assert.True(t, hnd.usesStreamer)
		assert.Equal(t, reflect.TypeOf(ResponseTypeStreamer{}), hnd.streamerType)
		assert.Zero(t, hnd.inType)
		assert.Zero(t, hnd.outType)
	})
}

type fakeServer struct{}

func (f fakeServer) headersTimeout() time.Duration                 { return 15 * time.Second }
func (f fakeServer) handlerForID(u uint64) (*serviceHandler, bool) { return nil, false }
func (f fakeServer) allMiddlewares() []Middleware                  { return nil }
func (f fakeServer) notifyClosed(c *srvConn)                       {}

func makeConnection() *srvConn {
	r, w := net.Pipe()
	go func() {
		_, _ = io.Copy(io.Discard, r)
	}()
	return &srvConn{
		server: fakeServer{},
		rw:     w,
		mu:     &sync.Mutex{},
		state:  0,
	}
}

func TestServerApplyReflect(t *testing.T) {
	t.Run("void request, response, stream", func(t *testing.T) {
		invoked := false
		handler := func(ctx context.Context, headers Header, res *ResponseTypeStreamer) error {
			invoked = true
			assert.Equal(t, "yes", headers["test"])
			res.Push(&ResponseType{})
			return nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		c := makeConnection()
		ctx := context.Background()
		err := c.apply(hnd, ctx, map[string]string{"test": "yes"}, nil)
		require.NoError(t, err)
		assert.True(t, invoked)
	})

	t.Run("request, void response, no stream", func(t *testing.T) {
		invoked := false
		handler := func(ctx context.Context, headers Header, req *RequestType) (Header, error) {
			invoked = true
			assert.Equal(t, "yes", headers["test"])
			return nil, nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		c := makeConnection()
		ctx := context.Background()
		err := c.apply(hnd, ctx, map[string]string{"test": "yes"}, &RequestType{})
		assert.NoError(t, err)
		assert.True(t, invoked)
	})

	t.Run("void request, void response, no stream", func(t *testing.T) {
		invoked := false
		handler := func(ctx context.Context, headers Header) (Header, error) {
			invoked = true
			assert.Equal(t, "yes", headers["test"])
			return nil, nil
		}
		s := NewServer("")
		s.RegisterHandler(0, "", handler)
		hnd := s.handlers[0].handler
		c := makeConnection()
		ctx := context.Background()
		err := c.apply(hnd, ctx, map[string]string{"test": "yes"}, nil)
		assert.NoError(t, err)
		assert.True(t, invoked)
	})
}

type SimpleServerImpl struct {
	registeredClients int
}

func (s *SimpleServerImpl) RegisterUser(ctx context.Context, headers Header, req *SimpleRequest, out *SimpleResponseStreamer) error {
	s.registeredClients++
	if req.Name == "Vito" && req.Email == "hey@vito.io" {
		out.Headers().Set("Test", "OK")
	}
	for i := 0; i < s.registeredClients; i++ {
		out.Push(&SimpleResponse{
			ID: int32(s.registeredClients),
		})
	}
	return nil
}

func (s *SimpleServerImpl) DeregisterUser(ctx context.Context, headers Header, req *SimpleRequest) (Header, *SimpleResponse, error) {
	ret := s.registeredClients
	s.registeredClients--
	return nil, &SimpleResponse{ID: int32(ret)}, nil
}

func TestFullServer(t *testing.T) {
	t.Cleanup(resetRegistry)
	v, err := os.CreateTemp("", "yarp-test")
	require.NoError(t, err)
	err = os.Remove(v.Name())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(v.Name())
	})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = l.Close()
	})
	srv := SimpleServerImpl{}
	s := NewServer("unix://" + v.Name())
	RegisterSimpleService(s, &srv)
	go func() {
		err := s.StartListener(l)
		assert.NoError(t, err)
	}()
	RegisterMessages()
	c := NewSimpleServiceClient(l.Addr().String())
	ch, headers, err := c.RegisterUser(context.Background(), &SimpleRequest{
		Name:  "Vito",
		Email: "hey@vito.io",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "OK", headers.Get("Test"))
	val, ok := <-ch
	assert.True(t, ok)
	assert.Equal(t, int32(1), val.ID)
}
