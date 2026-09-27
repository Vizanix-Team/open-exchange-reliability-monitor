package bybit

import (
	"errors"
	"io"
)

var errResponseTooLarge = errors.New("bybit: response exceeded size limit")

// readBounded reads at most limit+1 bytes and errors if the body was
// larger than limit, so a malicious or misbehaving endpoint can't exhaust
// probe memory. See SECURITY.md "oversized WebSocket messages,
// decompression bombs".
func readBounded(r io.Reader, limit int64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: limit + 1}
	buf, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > limit {
		return nil, errResponseTooLarge
	}
	return buf, nil
}
