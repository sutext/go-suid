package suid

import (
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"time"

	"sutext.github.io/suid/internal/coder"
)

// GUID is a globally unique identifier.
//
// The GUID is a 10-byte array that is unique across time and space.
// The first byte is the group ID, the next 6 bytes are the timestamp in microseconds,
// the next 2 bytes are the sequence number, and the last byte is the host ID.
// Waring: Do not modify the GUID manually.
type GUID [10]byte

// Group is the group ID of the GUID.
// User can define their own group ID.
type Group uint8

const (
	MAX_GROUP Group  = 0x0f
	MAX_SEQ   uint64 = 0x3fff
	MAX_TIME  int64  = 0x3f_ffff_ffff_ffff
)

var (
	_seq atomic.Uint64
)

// New creates a new GUID for the given group.
//
// If no group is provided, the default group is used.
//
// If the group is invalid, a panic is raised.
func New(group ...Group) (u GUID) {
	var g Group
	if len(group) > 0 {
		g = group[0]
	}
	if g > MAX_GROUP {
		panic(fmt.Sprintf("[GUID] Invalid input group: %d", g))
	}
	time := time.Now().UnixMicro()
	seq := _seq.Add(1) % MAX_SEQ
	u[0] = byte(g)<<4 | (byte(time>>50) & 0x0f)
	u[1] = byte(time >> 42)
	u[2] = byte(time >> 34)
	u[3] = byte(time >> 26)
	u[4] = byte(time >> 18)
	u[5] = byte(time >> 10)
	u[6] = byte(time >> 2)
	u[7] = byte(time<<6&0xc0) | byte(seq>>8&0x3f)
	u[8] = byte(seq)
	u[9] = coder.HOSTID
	return u
}

// Parse parses a GUID from a string.
//
// If the string is invalid, an error is returned.
//
// If the string is invalid, an error is returned.
func Parse(str string) (u GUID, err error) {
	err = u.decodeFrom([]byte(str))
	return u, err
}

// String returns the string representation of the GUID.
func (g GUID) String() string {
	bytes := make([]byte, 16)
	g.encodeInto(bytes)
	return string(bytes)
}

// Group returns the group ID.
func (g GUID) Group() Group {
	return Group(g[0] >> 4)
}

// HostID returns the host ID.
func (g GUID) HostID() uint8 {
	return g[9]
}

// Seq returns the sequence number.
func (g GUID) Seq() uint64 {
	return uint64(g[7]&0x3f)<<8 | uint64(g[8])
}

// Time returns the timestamp in microseconds.
func (g GUID) Time() int64 {
	return int64(g[0]&0x0f)<<50 | int64(g[1])<<42 | int64(g[2])<<34 | int64(g[3])<<26 | int64(g[4])<<18 | int64(g[5])<<10 | int64(g[6])<<2 | int64(g[7]>>6&0x3f)
}
func (g GUID) Verify() bool {
	return g.Group() <= MAX_GROUP && g.Seq() <= MAX_SEQ && g.Time() >= 1770904743122773
}

// Description returns a human-readable description of the GUID.
func (g GUID) Description() string {
	return fmt.Sprintf("group: %d, host: %d, seq: %d, time: %v", g.Group(), g.HostID(), g.Seq(), time.UnixMicro(g.Time()))
}

// MarshalJSON implements the json.Marshaler interface.
func (g GUID) MarshalJSON() ([]byte, error) {
	result := make([]byte, 18)
	result[0] = '"'
	g.encodeInto(result[1:])
	result[17] = '"'
	return result, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (g *GUID) UnmarshalJSON(data []byte) error {
	if len(data) != 18 || data[0] != '"' || data[17] != '"' {
		return fmt.Errorf("invalid guid json string")
	}
	return g.UnmarshalText(data[1:17])
}

// MarshalText implements the encoding.TextMarshaler interface.
func (g GUID) MarshalText() ([]byte, error) {
	bytes := make([]byte, 16)
	g.encodeInto(bytes)
	return bytes, nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (g *GUID) UnmarshalText(data []byte) error {
	return g.decodeFrom(data)
}

// Value implements the driver.Valuer interface.
func (g GUID) Value() (driver.Value, error) {
	return g.String(), nil
}

// Scan implements the sql.Scanner interface.
func (g *GUID) Scan(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("unsupported type for GUID: %T", value)
	}
	u, err := Parse(str)
	if err != nil {
		return err
	}
	*g = u
	return nil
}

func (g GUID) encodeInto(bytes []byte) {
	bytes[0] = coder.ENCODE[g[0]>>3]
	bytes[1] = coder.ENCODE[g[0]<<2&0x1f|g[1]>>6]
	bytes[2] = coder.ENCODE[g[1]>>1&0x1f]
	bytes[3] = coder.ENCODE[g[1]<<4&0x1f|g[2]>>4]
	bytes[4] = coder.ENCODE[g[2]<<1&0x1f|g[3]>>7]
	bytes[5] = coder.ENCODE[g[3]>>2&0x1f]
	bytes[6] = coder.ENCODE[g[3]<<3&0x1f|g[4]>>5]
	bytes[7] = coder.ENCODE[g[4]&0x1f]

	bytes[8] = coder.ENCODE[g[5]>>3]
	bytes[9] = coder.ENCODE[g[5]<<2&0x1f|g[6]>>6]
	bytes[10] = coder.ENCODE[(g[6]>>1)&0x1f]
	bytes[11] = coder.ENCODE[g[6]<<4&0x1f|g[7]>>4]
	bytes[12] = coder.ENCODE[g[7]<<1&0x1f|g[8]>>7]
	bytes[13] = coder.ENCODE[g[8]>>2&0x1f]
	bytes[14] = coder.ENCODE[g[8]<<3&0x1f|g[9]>>5]
	bytes[15] = coder.ENCODE[g[9]&0x1f]
}

// decode decodes a base32-encoded string to a uint64 value.
func (g *GUID) decodeFrom(d []byte) error {
	if len(d) != 16 {
		return fmt.Errorf("invalid suid string length")
	}
	for i := range d {
		idx, ok := coder.DECODE[d[i]]
		if !ok {
			return fmt.Errorf("invalid character in uuid string: %c", d[i])
		}
		d[i] = idx
	}
	var u GUID
	u[0] = d[0]<<3 | d[1]>>2
	u[1] = d[1]<<6 | d[2]<<1 | d[3]>>4
	u[2] = d[3]<<4 | d[4]>>1
	u[3] = d[4]<<7 | d[5]<<2 | d[6]>>3
	u[4] = d[6]<<5 | d[7]
	u[5] = d[8]<<3 | d[9]>>2
	u[6] = d[9]<<6 | d[10]<<1 | d[11]>>4
	u[7] = d[11]<<4 | d[12]>>1
	u[8] = d[12]<<7 | d[13]<<2 | d[14]>>3
	u[9] = d[14]<<5 | d[15]
	*g = u
	return nil
}
