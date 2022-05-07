package yarp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"
)

type contextKey struct {
	name string
}

var (
	srvContextKey = &contextKey{"server"}
)

// NewServer creates a new YARP Server with a given bind address and options.
// When using Unix Domain Sockets, bind must begin with `unix://`, followed by
// the socket's path. Otherwise, an IP:PORT pair is required.
// For options, see WithTimeout and WithTLS.
// Returns a Server instance ready to listen for connections.
func NewServer(bind string, opts ...Option) *Server {
	o := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	s := &Server{
		address:     bind,
		network:     "tcp",
		tlsConfig:   o.tlsConfig,
		timeout:     o.timeout,
		waitClients: &sync.WaitGroup{},
		handlers:    map[uint64]*serviceHandler{},
		mu:          &sync.Mutex{},
		clients:     map[*srvConn]bool{},
	}

	if strings.HasPrefix(bind, "unix://") {
		s.network = "unix"
		s.address = strings.TrimPrefix(bind, "unix://")
	}

	return s
}

type handlerFunction struct {
	fn           reflect.Value
	usesStreamer bool
	streamerType reflect.Type
	inType       reflect.Type
	outType      reflect.Type
}

func (h handlerFunction) String() string {
	return fmt.Sprintf("handlerFunction{fn: %#v, usesStreamer: %t, streamerType: %s, inType: %s, outType: %s}",
		h.fn, h.usesStreamer, h.streamerType, h.inType, h.outType)
}

type serviceHandler struct {
	id      uint64
	name    string
	fqn     string
	handler handlerFunction
}

type internalServer interface {
	headersTimeout() time.Duration
	handlerForID(uint64) (*serviceHandler, bool)
	allMiddlewares() []Middleware
	notifyClosed(c *srvConn)
}

// Server represents a server object capable of routing incoming connections and
// requests.
type Server struct {
	address     string
	network     string
	tlsConfig   *tls.Config
	stopChan    chan bool
	stopping    bool
	timeout     time.Duration
	waitClients *sync.WaitGroup
	middlewares []Middleware
	handlers    map[uint64]*serviceHandler

	mu      *sync.Mutex
	clients map[*srvConn]bool
}

func (s *Server) headersTimeout() time.Duration {
	return s.timeout
}

func (s *Server) handlerForID(u uint64) (*serviceHandler, bool) {
	hnd, ok := s.handlers[u]
	return hnd, ok
}

func (s *Server) allMiddlewares() []Middleware {
	return s.middlewares
}

// Middleware is a simple function that takes an RPCRequest, and either returns
// the same request and no error, in case the server should continue processing
// it, or an error, in case the server should stop processing it.
// Middlewares may update fields and the context present in the RPCRequest
// object, and return an updated value to be passed to the rest or the responder
// chain.
type Middleware func(req *RPCRequest) (*RPCRequest, error)

// Use registers a given Middleware to be executed on new requests.
func (s *Server) Use(mid Middleware) {
	s.middlewares = append(s.middlewares, mid)
}

// Start creates a new net.Listener for the address provided to NewServer, and
// invokes StartListener with it. This function always returns an error, that
// may occur during the net.Listen (bind) operation, during the server
// execution, or an ErrServerClosed in case the server is shutdown.
func (s *Server) Start() error {
	listener, err := net.Listen(s.network, s.address)
	if err != nil {
		return err
	}
	return s.StartListener(listener)
}

// StartListener starts the select loop for a given net.Listener and delegates
// incoming connections to goroutines, invoking all required Middlewares and
// handler functions. This function always returns an error, be during the
// server execution, or when Shutdown is called.
func (s *Server) StartListener(listener net.Listener) error {
	if s.tlsConfig != nil {
		listener = tls.NewListener(listener, s.tlsConfig)
	}
	if s.timeout == 0 {
		s.timeout = 15 * time.Second
	}
	s.stopChan = make(chan bool)
	var tmpDelay time.Duration
	baseContext := context.WithValue(context.Background(), srvContextKey, s)
	for {
		rw, err := listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return ErrServerClosed
			default:
			}
			if _, ok := err.(net.Error); ok {
				if tmpDelay == 0 {
					tmpDelay = 5 * time.Millisecond
				} else {
					tmpDelay *= 2
				}
				if max := 1 * time.Second; tmpDelay > max {
					tmpDelay = max
				}
				time.Sleep(tmpDelay)
				continue
			}
			return err
		}
		connCtx := baseContext
		tmpDelay = 0
		c := s.newConn(rw)
		go c.serve(connCtx)
	}
}

func isStreamer(t reflect.Type) bool {
	if t.Kind() != reflect.Pointer {
		return false
	}
	t = t.Elem()
	if t.NumMethod() != 2 {
		return false
	}
	if headers, push := t.Method(0), t.Method(1); headers.Name != "Headers" ||
		push.Name != "Push" ||
		push.Type.NumIn() != 2 ||
		push.Type.NumOut() != 0 ||
		headers.Type.NumIn() != 1 ||
		headers.Type.NumOut() != 1 ||
		headers.Type.Out(0) != reflectedHeaderType {
		return false
	} else if pushIn := push.Type.In(1); !canEncode(pushIn) {
		return false
	}
	if t.NumField() != 2 {
		return false
	}
	if h, ch := t.Field(0), t.Field(1); h.Name != "h" ||
		ch.Name != "ch" ||
		h.Type != reflectedHeaderType ||
		ch.Type.Kind() != reflect.Chan ||
		ch.Type.ChanDir() != reflect.SendDir {
		return false
	}
	return true
}

// RegisterHandler registers a given handler identified by k, and named by n,
// having a given handler function. This function is not intended to be used
// directly by users, but rather for autogenerated code responsible for
// registering a given service on a Server instance.
func (s *Server) RegisterHandler(k uint64, n string, handler interface{}) {
	fnVal := reflect.ValueOf(handler)
	fnType := fnVal.Type()
	if fnType.Kind() != reflect.Func {
		panic("yarp: RegisterHandler without a function")
	}

	numIn := fnType.NumIn()
	numOut := fnType.NumOut()
	if numIn < 2 || numIn > 4 || numOut == 0 || numOut > 3 {
		panic("yarp: RegisterHandler with incompatible handler function")
	}
	fn := handlerFunction{
		fn:           fnVal,
		usesStreamer: numOut == 1 && isStreamer(fnType.In(numIn-1)),
	}

	// When a streamer is used, the only return value possible is an error. If
	// the argument before the streamer is a header, request type is void.
	// Otherwise, the n-1 item is the request type.
	if fn.usesStreamer {
		fn.streamerType = fnType.In(numIn - 1).Elem()
		if fnType.In(numIn-2) != reflectedHeaderType {
			fn.inType = fnType.In(numIn - 2)
		}
	} else {
		if fnType.In(numIn-1) != reflectedHeaderType {
			fn.inType = fnType.In(numIn - 1)
		}
		if numOut == 3 {
			fn.outType = fnType.Out(1)
		}
	}

	c := strings.Split(n, ".")
	s.handlers[k] = &serviceHandler{
		id:      k,
		name:    c[len(c)-1],
		fqn:     n,
		handler: fn,
	}
}

func (s *Server) newConn(rw net.Conn) *srvConn {
	s.waitClients.Add(1)
	c := &srvConn{
		server: s,
		rw:     rw,
		mu:     &sync.Mutex{},
	}
	s.mu.Lock()
	s.clients[c] = true
	s.mu.Unlock()
	return c
}

// Shutdown prevents the current Server from accepting new connections, and
// waits until all current clients disconnects, or the provided ctx expires. In
// case ctx expires before all clients are finished, remaining clients will be
// forcefully disconnected. Passing a context without a timeout waits
// indefinitely for clients to finish.
func (s *Server) Shutdown(ctx context.Context) {
	if s.stopping {
		return
	}
	s.stopping = true
	s.stopChan <- true
	close(s.stopChan)
	poll := time.NewTicker(1 * time.Second)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			s.forceShutdown()
			return
		case <-poll.C:
			if len(s.clients) == 0 {
				return
			}
		}
	}
}

func (s *Server) forceShutdown() {
	s.mu.Lock()
	for c := range s.clients {
		c.close()
	}
	s.mu.Unlock()
}

func (s *Server) notifyClosed(c *srvConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c)
	s.waitClients.Done()
}

type connState int

const (
	connStateNew connState = iota
	connStateWaitingHeaders
	connStateReceivedHeaders
	connStateReceivingBody
	connStateReceivedBody
	connStateWritingResponse
	connStateWroteResponse
	connStateClosed
)

type srvConn struct {
	server internalServer
	rw     net.Conn
	mu     *sync.Mutex
	state  connState
}

func (c *srvConn) setState(new connState) {
	if new > c.state {
		c.state = new
	}
}

func (c *srvConn) serve(ctx context.Context) {
	defer func() {
		if err := recover(); err != nil {
			if e, ok := err.(error); ok {
				c.handleError(e)
				return
			}
			c.handleError(fmt.Errorf("panic on non-error value: %s", err))
		}
	}()

	c.setState(connStateWaitingHeaders)
	headersTimeout := time.NewTimer(c.server.headersTimeout())
	var request *Request
	reqChan := make(chan *Request)
	go c.readHeader(reqChan)
	select {
	case <-headersTimeout.C:
		c.close()
		return
	case req := <-reqChan:
		headersTimeout.Stop()
		if req == nil {
			c.close()
			return
		}
		c.setState(connStateReceivedHeaders)
		request = req
	}

	handler, ok := c.server.handlerForID(request.Method)
	if !ok {
		c.handleError(Error{
			Kind: ErrorKindUnimplementedMethod,
		})
		return
	}

	req := &RPCRequest{
		ctx:        ctx,
		Method:     handler.name,
		Identifier: handler.id,
		MethodFQN:  handler.fqn,
		Headers:    request.Headers,
	}

	for _, m := range c.server.allMiddlewares() {
		var err error
		req, err = m(req)
		if err != nil {
			c.handleError(err)
			return
		}
	}
	c.setState(connStateReceivingBody)
	_, data, err := Decode(c.rw)
	if err != nil {
		c.handleError(err)
		return
	}
	c.setState(connStateReceivedBody)
	if err = c.apply(handler.handler, req.ctx, req.Headers, data); err != nil {
		c.handleError(err)
		return
	}
	c.close()
}

func (c srvConn) readHeader(ch chan<- *Request) {
	req := Request{}
	defer close(ch)
	if err := req.Decode(c.rw); err != nil {
		ch <- nil
		return
	}
	ch <- &req
}

func (c *srvConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == connStateClosed {
		return
	}
	c.state = connStateClosed
	_ = c.rw.Close()
	go c.server.notifyClosed(c)
}

func (c *srvConn) handleError(err error) {
	defer c.close()
	var managed Error
	if man, ok := err.(Error); ok {
		managed = man
	} else {
		managed = Error{
			Kind:       ErrorKindInternalError,
			Headers:    nil,
			Identifier: "",
			UserData:   nil,
		}
	}

	// TODO: Log, report?

	// There's no point in writing an error value in case c's state does not
	// match the following condition.
	if c.state >= connStateReceivedHeaders && c.state < connStateWritingResponse {
		output, err := managed.Encode()
		if err != nil {
			// Oh well, this is unfortunate...
			return
		}
		_, _ = io.Copy(c.rw, bytes.NewReader(output))
	}
}

func (c *srvConn) apply(handler handlerFunction, ctx context.Context, h Header, data interface{}) error {
	applyParams := make([]reflect.Value, 0, 4)
	applyParams = append(applyParams, reflect.ValueOf(ctx), reflect.ValueOf(h))
	if handler.inType != nil {
		if data == nil {
			return Error{Kind: ErrorKindTypeMismatch}
		}
		dataVal := reflect.ValueOf(data)
		dataTyp := dataVal.Type()
		if handler.inType.Kind() == reflect.Pointer && dataTyp.Kind() != reflect.Pointer {
			ptr := reflect.New(dataTyp)
			ptr.Elem().Set(dataVal)
			dataTyp = ptr.Type()
			dataVal = ptr
		}
		if !dataTyp.ConvertibleTo(handler.inType) {
			return Error{Kind: ErrorKindTypeMismatch}
		}
		applyParams = append(applyParams, dataVal.Convert(handler.inType))
	}

	if handler.usesStreamer {
		vPtr := reflect.New(handler.streamerType)
		tChan := reflect.ChanOf(reflect.BothDir, handler.streamerType.Field(1).Type.Elem())
		vChan := reflect.MakeChan(tChan, 10)
		hVal := reflect.ValueOf(h)
		v := vPtr.Elem()
		reflect.NewAt(v.Field(0).Type(), unsafe.Pointer(v.Field(0).UnsafeAddr())).Elem().Set(hVal)
		reflect.NewAt(v.Field(1).Type(), unsafe.Pointer(v.Field(1).UnsafeAddr())).Elem().Set(vChan)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go c.serviceStreamer(vChan, h, wg.Done)

		applyParams = append(applyParams, vPtr)
		retVals := handler.fn.Call(applyParams)
		vChan.Close()
		wg.Wait()
		if !retVals[0].IsNil() {
			if err := retVals[0].Interface().(error); err != nil {
				return err
			}
		}
		c.setState(connStateWroteResponse)
		return nil
	}

	retVal := handler.fn.Call(applyParams)
	errVal := retVal[len(retVal)-1]
	if !errVal.IsNil() {
		if err := errVal.Interface().(error); err != nil {
			return err
		}
	}

	respHeaders := retVal[len(retVal)-2].Interface().(Header)
	var respData []byte
	var err error
	if handler.outType == nil {
		respData = encodeVoid()
	} else if respData, err = encode(retVal[0]); err != nil {
		return err
	}
	if err = c.writeResponseHeader(respHeaders, false); err != nil {
		return err
	}
	_, err = io.Copy(c.rw, bytes.NewReader(respData))
	return err
}

func (c *srvConn) serviceStreamer(stream reflect.Value, h Header, done func()) {
	errored := false
	for {
		v, ok := stream.Recv()
		if !ok {
			break
		}
		if errored {
			continue
		}
		if c.state == connStateReceivedBody {
			// Flush headers
			if err := c.writeResponseHeader(h, true); err != nil {
				c.handleError(err)
				errored = true
				continue
			}
		}
		data, err := encode(v)
		if err != nil {
			c.handleError(err)
			errored = true
			continue
		}
		_, err = io.Copy(c.rw, bytes.NewBuffer(data))
		if err != nil {
			c.handleError(err)
			errored = true
		}
	}
	done()
}

func (c *srvConn) writeResponseHeader(headers Header, streaming bool) error {
	data, err := Response{headers, streaming}.Encode()
	if err != nil {
		return err
	}
	buf := bytes.NewReader(data)
	_, err = io.Copy(c.rw, buf)
	if err == nil {
		c.state = connStateWritingResponse
	}
	return err
}

// RPCRequest represents an incoming RPC request.
// Although fields like Method, and Identifier are available, changing them
// have no effect other than passing them down to other middlewares. Changing
// the context using WithContext, however, causes the new context to be provided
// to the target Handler. The same applies to Headers.
type RPCRequest struct {
	ctx        context.Context
	Method     string
	Identifier uint64
	MethodFQN  string
	Headers    Header
}

// Context returns the context for the current RPCRequest. To update the
// context, use WithContext to obtain a new RPCRequest instance that must be
// returned from the Middleware function.
func (r RPCRequest) Context() context.Context {
	return r.ctx
}

// WithContext returns a new RPCRequest instance using the given context as its
// context.
func (r RPCRequest) WithContext(ctx context.Context) *RPCRequest {
	return &RPCRequest{
		ctx:        ctx,
		Method:     r.Method,
		Headers:    r.Headers,
		Identifier: r.Identifier,
		MethodFQN:  r.MethodFQN,
	}
}
