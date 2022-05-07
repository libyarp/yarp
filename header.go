package yarp

import (
	"net/textproto"
	"reflect"
)

// Header represents a list of headers present in requests and responses. It may
// be important to notice that headers manipulated by Get, Set, and Del have
// their keys automatically standardized using MIME Header standard. For further
// information regarding that conversion, see textproto.CanonicalMIMEHeaderKey.
type Header map[string]string

var reflectedHeaderType = reflect.TypeOf(Header{})

// Set inserts or replaces a given key in the header map with a given value.
func (h Header) Set(key, value string) {
	h[textproto.CanonicalMIMEHeaderKey(key)] = value
}

// Get either returns a header with a given key, or an empty string, in case it
// is absent.
func (h Header) Get(key string) string {
	return h[textproto.CanonicalMIMEHeaderKey(key)]
}

// Del removes a given key from the header list
func (h Header) Del(key string) {
	delete(h, textproto.CanonicalMIMEHeaderKey(key))
}

// Clone clones the current Header, returning a new instance.
func (h Header) Clone() Header {
	n := make(map[string]string, len(h))
	for k, v := range h {
		n[k] = v
	}
	return n
}
