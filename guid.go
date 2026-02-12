package suid

import (
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"time"
)

// GUID is a globally unique identifier.
//
// A GUID is a 10-byte array that is unique across time and space.
// The first byte is the group ID, the next 6 bytes are the timestamp in microseconds,
// the next 2 bytes are the sequence number, and the last byte is the host ID.
type GUID [10]byte

var (
	_seq     atomic.Int64
	_max_seq int64 = 0x1ffff
)

// NewGUID creates a new GUID for the given group.
//
// If no group is provided, the default group is used.
//
// If the group is invalid, a panic is raised.
func NewGUID(group ...uint8) (u GUID) {
	var g uint8
	if len(group) > 0 {
		g = group[0]
	}
	if g > uint8(MAX_GROUP) {
		panic(fmt.Sprintf("[SUID] Invalid input group: %d", g))
	}
	time := time.Now().UnixMicro()
	seq := _seq.Add(1) % _max_seq
	u[0] = g<<5 | (byte(time>>48) & 0x1f)
	u[1] = byte(time >> 40)
	u[2] = byte(time >> 32)
	u[3] = byte(time >> 24)
	u[4] = byte(time >> 16)
	u[5] = byte(time >> 8)
	u[6] = byte(time)
	u[7] = byte(seq >> 9)
	u[8] = byte(seq >> 1)
	u[9] = byte((seq << 7 & 0x80) | _HOST_ID)
	return u
}

// ParseGUID parses a GUID from a string.
//
// If the string is invalid, an error is returned.
//
// If the string is invalid, an error is returned.
func ParseGUID(str string) (u GUID, err error) {
	return decodeGUID([]byte(str))
}

// String returns the string representation of the GUID.
func (s GUID) String() string {
	return string(encodeGUID(s))
}

// Group returns the group ID.
func (s GUID) Group() uint8 {
	return s[0] >> 5
}

// HostID returns the host ID.
func (s GUID) HostID() int64 {
	return int64(s[9] & 0x7f)
}

// Seq returns the sequence number.
func (s GUID) Seq() int64 {
	return int64(s[7])<<9 | int64(s[8])<<1 | int64(s[9])>>7
}

// Time returns the timestamp in microseconds.
func (s GUID) Time() int64 {
	return int64(s[6]) | int64(s[5])<<8 | int64(s[4])<<16 | int64(s[3])<<24 | int64(s[2])<<32 | int64(s[1])<<40 | int64(s[0]&0x1f)<<48
}
func (s GUID) Verify() bool {
	return s.Group() <= uint8(MAX_GROUP) && s.HostID() <= MAX_HOST && s.Seq() <= MAX_SEQ && s.Time() >= 1770904743122773
}

// Description returns a human-readable description of the GUID.
func (s GUID) Description() string {
	return fmt.Sprintf("group: %d, host: %d, seq: %d, time: %d", s.Group(), s.HostID(), s.Seq(), s.Time())
}

// MarshalJSON implements the json.Marshaler interface.
func (s GUID) MarshalJSON() ([]byte, error) {
	result := make([]byte, 18)
	result[0] = '"'
	copy(result[1:], encodeGUID(s))
	result[17] = '"'
	return result, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *GUID) UnmarshalJSON(data []byte) error {
	if len(data) != 18 || data[0] != '"' || data[17] != '"' {
		return fmt.Errorf("invalid guid json string")
	}
	return s.UnmarshalText(data[1:17])
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s GUID) MarshalText() ([]byte, error) {
	return encodeGUID(s), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *GUID) UnmarshalText(data []byte) error {
	u, err := decodeGUID(data)
	if err != nil {
		return err
	}
	*s = u
	return nil
}

// Value implements the driver.Valuer interface.
func (s GUID) Value() (driver.Value, error) {
	return s.String(), nil
}

// Scan implements the sql.Scanner interface.
func (s *GUID) Scan(value any) error {
	switch v := value.(type) {
	case string:
		u, err := ParseGUID(v)
		if err != nil {
			return err
		}
		*s = u
		return nil
	default:
		return fmt.Errorf("unsupported type for SUID: %T", value)
	}
}

// GormDataType implements the gorm.DataTypeInterface interface.
func (GUID) GormDataType() string {
	return "string"
}

func encodeGUID(u GUID) []byte {
	bytes := make([]byte, 16)
	bytes[0] = _ENCODE[u[0]>>3]
	bytes[1] = _ENCODE[u[0]<<2&0x1f|u[1]>>6]
	bytes[2] = _ENCODE[u[1]>>1&0x1f]
	bytes[3] = _ENCODE[u[1]<<4&0x1f|u[2]>>4]
	bytes[4] = _ENCODE[u[2]<<1&0x1f|u[3]>>7]
	bytes[5] = _ENCODE[u[3]>>2&0x1f]
	bytes[6] = _ENCODE[u[3]<<3&0x1f|u[4]>>5]
	bytes[7] = _ENCODE[u[4]&0x1f]

	bytes[8] = _ENCODE[u[5]>>3]
	bytes[9] = _ENCODE[u[5]<<2&0x1f|u[6]>>6]
	bytes[10] = _ENCODE[(u[6]>>1)&0x1f]
	bytes[11] = _ENCODE[u[6]<<4&0x1f|u[7]>>4]
	bytes[12] = _ENCODE[u[7]<<1&0x1f|u[8]>>7]
	bytes[13] = _ENCODE[u[8]>>2&0x1f]
	bytes[14] = _ENCODE[u[8]<<3&0x1f|u[9]>>5]
	bytes[15] = _ENCODE[u[9]&0x1f]
	return bytes
}

// decode decodes a base32-encoded string to a uint64 value.
func decodeGUID(d []byte) (u GUID, err error) {
	if len(d) != 16 {
		return u, fmt.Errorf("invalid suid string length")
	}
	for i := range d {
		idx, ok := _DECODE[d[i]]
		if !ok {
			return u, fmt.Errorf("invalid character in uuid string: %c", d[i])
		}
		d[i] = idx
	}
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
	return u, nil
}
