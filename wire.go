package yarp

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
)

var (
	magicRequest  = []byte{0x79, 0x79, 0x72}
	magicResponse = []byte{0x79, 0x79, 0x52}
	magicError    = []byte{0x79, 0x79, 0x65}
)

// Request represents an internal representation of an incoming request through
// a stream. Method indicates which handler should be called, and Headers
// contains any metadata sent by a client.
type Request struct {
	Method  uint64
	Headers map[string]string
}

// Encode encodes the Request header into a byte slice
func (r Request) Encode() ([]byte, error) {
	header := encodeUint(r.Method)
	heads, err := encodeMap(reflect.ValueOf(r.Headers))
	if err != nil {
		return nil, err
	}
	data := append(magicRequest, encodeUint(uint64(len(header)+len(heads)))...)
	data = append(data, header...)
	return append(data, heads...), nil
}

// Decode reads from a given io.Reader the required bytes to compose a Request,
// and sets fields present in the receiver.
func (r *Request) Decode(re io.Reader) error {
	magic := make([]byte, 3)
	if _, err := io.ReadFull(re, magic); err != nil {
		return err
	}
	if !bytes.Equal(magic, magicRequest) {
		return ErrCorruptStream
	}

	head := []byte{0x00}
	if _, err := io.ReadFull(re, head); err != nil {
		return err
	}
	_, l, err := decodeScalar(head[0], re)
	if err != nil {
		return err
	}

	lr := io.LimitReader(re, int64(l))
	if _, err = io.ReadFull(lr, head); err != nil {
		return err
	}
	_, s, err := decodeScalar(head[0], lr)
	if err != nil {
		return err
	}
	r.Method = s

	if _, err = io.ReadFull(lr, head); err != nil {
		return err
	}
	h, err := decodeMap(head[0], lr)
	if err != nil {
		return err
	}
	str := reflect.TypeOf("")
	ok, mv := makeMap(h, reflect.MapOf(str, str))
	if !ok {
		return ErrCorruptStream
	}
	r.Headers = mv.Interface().(map[string]string)
	return nil
}

// Response indicates the beginning of a response in a YARP stream. The response
// contains a set of arbitrary headers, followed by a boolean value indicating
// whether the server will begin to provide a stream response comprised of
// potentially multiple objects.
type Response struct {
	Headers map[string]string
	Stream  bool
}

// Encode encodes a given Response structure into a byte slice.
func (r Response) Encode() ([]byte, error) {
	heads, err := encodeMap(reflect.ValueOf(r.Headers))
	if err != nil {
		return nil, err
	}
	str, err := encode(reflect.ValueOf(r.Stream))
	if err != nil {
		return nil, err
	}
	data := append(magicResponse, heads...)
	data = append(data, str...)
	return data, nil
}

// Decode reads all required bytes from a given io.Reader and fills the
// receiver's fields.
func (r *Response) Decode(re io.Reader) error {
	magic := make([]byte, 3)
	if _, err := io.ReadFull(re, magic); err != nil {
		return err
	}
	if !bytes.Equal(magic, magicResponse) {
		return ErrCorruptStream
	}

	head := []byte{0x00}
	if _, err := io.ReadFull(re, head); err != nil {
		return err
	}
	h, err := decodeMap(head[0], re)
	if err != nil {
		return err
	}
	str := reflect.TypeOf("")
	ok, mv := makeMap(h, reflect.MapOf(str, str))
	if !ok {
		return ErrCorruptStream
	}
	r.Headers = mv.Interface().(map[string]string)

	if _, err := io.ReadFull(re, head); err != nil {
		return err
	}

	s, _, err := decodeScalar(head[0], re)
	if err != nil {
		return err
	}
	r.Stream = s
	return nil
}

// ErrorKind indicates one of the possible errors returned in a YARP stream.
type ErrorKind uint

const (
	// ErrorKindInternalError indicates that an internal error prevented the
	// server from performing the requested operation.
	ErrorKindInternalError = 0

	// ErrorKindManagedError indicates that the server returned a user-defined
	// error. See Identifier and UserData fields, and consult the service's
	// documentation for further information.
	ErrorKindManagedError = 1

	// ErrorKindRequestTimeout indicates that the server reached a timeout while
	// waiting for the client to transmit headers.
	ErrorKindRequestTimeout = 2

	// ErrorKindUnimplementedMethod indicates that the server does not implement
	// the requested method.
	ErrorKindUnimplementedMethod = 3

	// ErrorKindTypeMismatch indicates that the contract between the server and
	// client is out-of-sync, since they could not agree on a type for either
	// a request or response.
	ErrorKindTypeMismatch = 4

	// ErrorKindUnauthorized indicates that the server refused to perform an
	// operation due to lack of authorization. See Identifier and UserData
	// fields, along with the service's documentation for further information.
	ErrorKindUnauthorized = 5

	// ErrorKindBadRequest indicates that the server refused to continue the
	// operation due to a problem with the incoming request. See Identifier and
	// UserData fields, along with the service's documentation for further
	// information.
	ErrorKindBadRequest = 6
)

var errorKindString = map[ErrorKind]string{
	ErrorKindInternalError:       "Internal Error",
	ErrorKindManagedError:        "Managed Error",
	ErrorKindRequestTimeout:      "Request Timeout",
	ErrorKindUnimplementedMethod: "Unimplemented Method",
	ErrorKindTypeMismatch:        "Type Mismatch",
	ErrorKindUnauthorized:        "Unauthorized",
	ErrorKindBadRequest:          "Bad Request",
}

// Error represents a handled error from the server or an underlying component.
// Error contains special fields that may be used to diagnose and/or handle the
// error, such as Kind (see ErrorKind documentation), an optional set of
// Headers, an optional Identifier provided by the service implementation, and
// an optional list of UserData key-values with potentially relevant data
// regarding the error. For further information, refer to the service's
// documentation.
type Error struct {
	Kind       ErrorKind
	Headers    Header
	Identifier string
	UserData   map[string]string
}

func (e Error) Error() string {
	heads := make([]string, 0, len(e.Headers))
	for k, v := range e.Headers {
		heads = append(heads, fmt.Sprintf("%s=%s", k, v))
	}
	ud := make([]string, 0, len(e.UserData))
	for k, v := range e.UserData {
		ud = append(ud, fmt.Sprintf("%s=%s", k, v))
	}

	desc, ok := errorKindString[e.Kind]
	if !ok {
		desc = "Invalid?"
	}
	c := []string{
		fmt.Sprintf("yarp/Error: Code %d (%s)", e.Kind, desc),
	}
	if e.Identifier != "" {
		c = append(c, e.Identifier)
	}
	if len(heads)+len(ud) > 0 {
		data := []string{"("}
		if len(heads) > 0 {
			data = append(data, "Headers: ", strings.Join(heads, ", "))
		}
		if len(ud) > 0 {
			data = append(data, "UserData: ", strings.Join(ud, ", "))
		}
		data = append(data, ")")
		c = append(c, strings.Join(data, ""))
	}

	return strings.Join(c, " ")
}

func (e Error) Encode() ([]byte, error) {
	kind := encodeUint(uint64(e.Kind))
	heads, err := encodeMap(reflect.ValueOf(e.Headers))
	if err != nil {
		return nil, err
	}
	id, err := encode(reflect.ValueOf(e.Identifier))
	if err != nil {
		return nil, err
	}
	ud, err := encodeMap(reflect.ValueOf(e.UserData))
	if err != nil {
		return nil, err
	}

	data := append(magicError, kind...)
	data = append(data, heads...)
	data = append(data, id...)
	data = append(data, ud...)
	return data, nil
}

func (e *Error) Decode(re io.Reader) error {
	magic := make([]byte, 3)
	if _, err := io.ReadFull(re, magic); err != nil {
		return err
	}
	if !bytes.Equal(magic, magicError) {
		return ErrCorruptStream
	}

	head := []byte{0x00}
	if _, err := io.ReadFull(re, head); err != nil {
		return err
	}
	_, v, err := decodeScalar(head[0], re)
	if err != nil {
		return err
	}
	e.Kind = ErrorKind(v)

	if _, err := io.ReadFull(re, head); err != nil {
		return err
	}
	h, err := decodeMap(head[0], re)
	if err != nil {
		return err
	}
	str := reflect.TypeOf("")
	ok, mv := makeMap(h, reflect.MapOf(str, str))
	if !ok {
		return ErrCorruptStream
	}
	e.Headers = mv.Interface().(map[string]string)

	if _, err := io.ReadFull(re, head); err != nil {
		return err
	}
	id, err := decodeString(head[0], re)
	if err != nil {
		return err
	}
	e.Identifier = id

	if _, err := io.ReadFull(re, head); err != nil {
		return err
	}
	h, err = decodeMap(head[0], re)
	if err != nil {
		return err
	}
	ok, mv = makeMap(h, reflect.MapOf(str, str))
	if !ok {
		return ErrCorruptStream
	}
	e.UserData = mv.Interface().(map[string]string)
	return nil
}
