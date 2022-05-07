package yarp

import "io"

func encodeString(str string) []byte {
	header := encodeInteger(uint64(len(str)))
	header[0] |= 0xA0
	return append(header, []byte(str)...)
}

func decodeString(header byte, r io.Reader) (string, error) {
	_, size, err := decodeScalar(header, r)
	if err != nil {
		return "", nil
	}
	if size >= sizeLimit {
		return "", ErrSizeTooLarge
	}
	r = io.LimitReader(r, int64(size))
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
