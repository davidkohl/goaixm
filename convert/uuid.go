package convert

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

// UUID is an RFC 4122 UUID, used for the identities of converted features.
type UUID [16]byte

// NewV5 returns the name-based (SHA-1, version 5) UUID of name in the
// namespace ns.  Equal inputs give equal UUIDs, which makes converted
// identities stable across conversions.
func NewV5(ns UUID, name string) UUID {
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	var u UUID
	copy(u[:], h.Sum(nil))
	u[6] = u[6]&0x0f | 0x50 // version 5
	u[8] = u[8]&0x3f | 0x80 // RFC 4122 variant
	return u
}

// ParseUUID parses the canonical form "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
// (either case), optionally prefixed with "urn:uuid:".
func ParseUUID(s string) (UUID, error) {
	var u UUID
	t := strings.TrimPrefix(strings.TrimSpace(s), "urn:uuid:")
	if len(t) != 36 || t[8] != '-' || t[13] != '-' || t[18] != '-' || t[23] != '-' {
		return u, fmt.Errorf("convert: invalid UUID %q", s)
	}
	h := t[0:8] + t[9:13] + t[14:18] + t[19:23] + t[24:]
	if _, err := hex.Decode(u[:], []byte(h)); err != nil {
		return UUID{}, fmt.Errorf("convert: invalid UUID %q", s)
	}
	return u, nil
}

// IsZero reports whether u is the nil UUID.
func (u UUID) IsZero() bool { return u == UUID{} }

// String returns the canonical lower-case form.
func (u UUID) String() string {
	var b [36]byte
	hex.Encode(b[0:8], u[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], u[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], u[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], u[8:10])
	b[23] = '-'
	hex.Encode(b[24:], u[10:])
	return string(b[:])
}

// MarshalText encodes the canonical form.
func (u UUID) MarshalText() ([]byte, error) { return []byte(u.String()), nil }

// UnmarshalText parses the canonical form.
func (u *UUID) UnmarshalText(b []byte) error {
	p, err := ParseUUID(string(b))
	if err != nil {
		return err
	}
	*u = p
	return nil
}
