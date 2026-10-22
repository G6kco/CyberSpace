package users

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// crockford is the ULID alphabet: no I, L, O or U.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewPublicID returns a ULID: 48 bits of milliseconds then 80 random bits,
// as the 26 characters users.public_id holds.
func NewPublicID(now time.Time) string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], uint64(now.UnixMilli())<<16)
	_, _ = rand.Read(b[6:])

	hi := binary.BigEndian.Uint64(b[:8])
	lo := binary.BigEndian.Uint64(b[8:])
	var out [26]byte
	// 128 bits in 26 five-bit characters: the first carries only three.
	for i := 25; i >= 0; i-- {
		out[i] = crockford[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out[:])
}
