package suid

import (
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"time"
)

type UUID [10]byte

const (
	MAX_UUID_GROUP uint8 = 0x3f
)

var (
	_seq = atomic.Uint32{}
)

func NewUUID(group ...uint8) (u UUID) {
	var g uint8
	if len(group) > 0 {
		g = group[0]
	}
	time := time.Now().Unix()
	u[0] = g<<2 | (byte(time>>32) & 0x3)
	u[1] = byte(time >> 24)
	u[2] = byte(time >> 16)
	u[3] = byte(time >> 8)
	u[4] = byte(time)
	seq := _seq.Add(1)
	u[5] = byte(seq >> 24)
	u[6] = byte(seq >> 16)
	u[7] = byte(seq >> 8)
	u[8] = byte(seq)
	u[9] = byte(_HOST_ID)
	return u
}
func ParseUUID(str string) (u UUID, err error) {
	return decodeUUID([]byte(str))
}
func (s UUID) String() string {
	return string(encodeUUID(s))
}
func (s UUID) Group() uint8 {
	return s[0] >> 2
}
func (s UUID) HostID() int64 {
	return int64(s[9])
}
func (s UUID) Seq() int64 {
	return int64(s[8]) | int64(s[7])<<8 | int64(s[6])<<16 | int64(s[5])<<24
}
func (s UUID) Time() int64 {
	return int64(s[4]) | int64(s[3])<<8 | int64(s[2])<<16 | int64(s[1])<<24 | int64(s[0]&0x3)<<32
}
func (s UUID) Description() string {
	return fmt.Sprintf("group: %d, host: %d, seq: %d, time: %d", s.Group(), s.HostID(), s.Seq(), s.Time())
}

// MarshalJSON implements the json.Marshaler interface.
func (s UUID) MarshalJSON() ([]byte, error) {
	result := make([]byte, 18)
	result[0] = '"'
	copy(result[1:], encodeUUID(s))
	result[17] = '"'
	return result, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *UUID) UnmarshalJSON(data []byte) error {
	if len(data) != 18 || data[0] != '"' || data[14] != '"' {
		return fmt.Errorf("invalid suid json string")
	}
	return s.UnmarshalText(data[1:17])
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s UUID) MarshalText() ([]byte, error) {
	return encodeUUID(s), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *UUID) UnmarshalText(data []byte) error {
	u, err := decodeUUID(data)
	if err != nil {
		return err
	}
	*s = u
	return nil
}

// Value implements the driver.Valuer interface.
func (s UUID) Value() (driver.Value, error) {
	return s.String(), nil
}

// Scan implements the sql.Scanner interface.
func (s *UUID) Scan(value any) error {
	switch v := value.(type) {
	case string:
		u, err := ParseUUID(v)
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
func (UUID) GormDataType() string {
	return "string"
}

func encodeUUID(u UUID) []byte {
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
func decodeUUID(d []byte) (u UUID, err error) {
	if len(d) != 16 {
		return u, fmt.Errorf("invalid suid string length")
	}
	u[0] = _DECODE[d[0]<<3|d[1]>>2]
	u[1] = _DECODE[d[1]<<6|d[2]<<1|d[3]>>4]
	u[2] = _DECODE[d[3]<<4|d[4]>>1]
	u[3] = _DECODE[d[4]<<7|d[5]<<2|d[6]>>3]
	u[4] = _DECODE[d[6]<<5|d[7]]

	u[5] = _DECODE[d[8]<<3|d[9]>>2]
	u[6] = _DECODE[d[9]<<6|d[10]<<1|d[11]>>4]
	u[7] = _DECODE[d[11]<<4|d[12]>>1]
	u[8] = _DECODE[d[12]<<7|d[13]<<2|d[14]>>3]
	u[9] = _DECODE[d[14]<<5|d[15]]
	return u, nil
}
