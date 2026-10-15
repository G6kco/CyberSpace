// Package evaluation checks submitted answers against stored validators.
package evaluation

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"strings"
)

// MinKeyLength is the shortest accepted answer key, in bytes.
const MinKeyLength = 32

// ErrShortKey means the answer key is too short to keep answers secret.
var ErrShortKey = errors.New("answer key must be at least 32 bytes")

// Digest is the stored form of an expected answer: HMAC-SHA256 under the
// server's answer key, so a database dump does not reveal any flag and the
// short numeric answers cannot be brute-forced without the key.
//
// Answers are compared case-insensitively with surrounding space removed:
// the question bank mixes case in hex flags ("D935e3...") and in words
// ("TCP", "green"), and a stray space is never part of a flag.
func Digest(key []byte, answer string) [32]byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(Normalize(answer)))
	var sum [32]byte
	copy(sum[:], mac.Sum(nil))
	return sum
}

// Normalize is the form an answer is compared in.
func Normalize(answer string) string {
	return strings.ToLower(strings.TrimSpace(answer))
}

// Matches reports whether answer is the one whose digest is expected, in
// constant time.
func Matches(key []byte, answer string, expected []byte) bool {
	got := Digest(key, answer)
	return hmac.Equal(got[:], expected)
}
