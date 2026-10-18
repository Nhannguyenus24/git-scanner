// Package id generates UUIDv7 identifiers using the standard library only.
//
// Version 7 rather than 4 because the first 48 bits are a millisecond timestamp,
// so generated ids sort by creation time. The findings table is insert-heavy and
// its primary key index stays append-mostly instead of scattering writes across
// the whole b-tree, which is what random v4 keys do.
package id

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/mservice/gitscanner/libs/entity"
)

// New returns a UUIDv7 in canonical string form.
//
// It panics only if the system CSPRNG fails, which on a running process means
// something is badly wrong and silently returning a weak id would be worse.
func New() entity.ID {
	var b [16]byte

	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	if _, err := rand.Read(b[6:]); err != nil {
		panic("id: crypto/rand failed: " + err.Error())
	}

	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant

	return entity.ID(format(b))
}

func format(b [16]byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}
