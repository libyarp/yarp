package yarp

import (
	"fmt"
	"reflect"
)

// ErrNonHomogeneousArray indicates that an operation was attempted against a
// non-homogeneous array or slice. Only homogeneous arrays are supported by
// YARP.
var ErrNonHomogeneousArray = fmt.Errorf("only homogeneous arrays are supported")

// ErrSizeTooLarge indicates that a message is either too large, or its stream
// is corrupted. The size limitation of 2GB was arbitrarily imposed to detect
// faulty messages.
var ErrSizeTooLarge = fmt.Errorf("size is too large")

// ErrInvalidType indicates that an invalid type was encountered in the stream,
// possibly indicating that it is corrupted.
var ErrInvalidType = fmt.Errorf("invalid type in stream")

// ErrIncompatibleStruct indicates that a struct that does not implement the
// StructValuer interface was provided.
var ErrIncompatibleStruct = fmt.Errorf("incompatible structure to encode; structures must implement StructValuer")

// ErrIncompleteStruct indicates that a struct lacking a *Structure field was
// provided.
var ErrIncompleteStruct = fmt.Errorf("incomplete structure definition; missing *Structure field")

// ErrInvalidTag indicates that a structure tag contains an invalid value that
// could not be parsed as a number.
var ErrInvalidTag = fmt.Errorf("invalid index tag")

// ErrDuplicatedFieldIndex indicates that a given structure contains one or more
// fields sharing the same index tag.
var ErrDuplicatedFieldIndex = fmt.Errorf("duplicated field index")

// ErrMinFieldNotZero indicates that a given structure does not have a field
// beginning at zero.
var ErrMinFieldNotZero = fmt.Errorf("minimum field index should be zero")

// ErrFieldGap indicates that a given structure contains one or more gaps
// between fields indexes. The field list indexes must be contiguous.
var ErrFieldGap = fmt.Errorf("structs must have no gaps between field indexes")

// ErrUnknownStructType indicates that the message being parsed refers to an
// unknown struct type.
var ErrUnknownStructType = fmt.Errorf("unknown struct type")

// ErrCorruptStream indicates that the stream being processed is corrupt.
var ErrCorruptStream = fmt.Errorf("corrupt stream")

// ErrWantsStreamed indicates that the client and server implementation is
// possibly out-of-sync, since the server intends to return a streamed response
// whilst the client wants a single one.
var ErrWantsStreamed = fmt.Errorf("method requires a streamed response")

// ErrServerClosed indicates that the server was closed. This error is returned
// by Server's Start and StartListener methods when Shutdown is called.
var ErrServerClosed = fmt.Errorf("server closed")

// IsManagedError indicates whether a given error value can be converted to an
// Error instance, and returns it, in case conversion is possible.
func IsManagedError(err error) (bool, Error) {
	man, ok := err.(Error)
	return ok, man
}

// IncompatibleTypeError indicates that the server returned an unexpected type
// as a response for a method.
type IncompatibleTypeError struct {
	Received interface{}
	Wants    reflect.Type
}

func (i IncompatibleTypeError) Error() string {
	return fmt.Sprintf("received incompatible type as response: %T, wants %s", i.Received, i.Wants)
}
