package suid

import (
	"fmt"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"sutext.github.io/suid/internal/coder"
)

// Snowflake Unique Identifier SUID is a 64-bit integer.
// The data structure is symbol(1)-group(3)-time(34)-seq(19)-host(7)
//   - The host_id will read `SUID_HOST_ID` `POD_NAME` `HOSTNAME` from environment, and pick last part ( separator "-" )
//   - If run in kubenetes, recommend to use StatefulSet to provide a unique host_id.
//   - Otherwise, you must set `SUID_HOST_ID` manually to provide a unique host_id.
//   - The max date for SUID will be `2514-05-30 01:53:03 +0000`
//   - The max number of concurrent transactions is 524288 per second. It will wait for next second automatically
//   - Recommend to use SUID as primary key for database table to ensure the uniqueness and performance.
//   - The SUID will keep threaded-safe and safe for concurrent access.
type SUID uint64
type Group uint8

const (
	MAX_GROUP Group = 0x7         // 3 bits
	MAX_SEQ   int64 = 0x7ffff     // 19 bits
	MAX_TIME  int64 = 0x3ffffffff // 34 bits
)

var (
	_WID_SEQ  = bitWidth(MAX_SEQ)    //SEQ bits width
	_WID_HOST = 8                    //HOST bits width
	_WID_TIME = bitWidth(MAX_TIME)   //TIME bits width
	_Builders = map[Group]*builder{} // group -> builder
)

func init() {
	_Builders = make(map[Group]*builder, MAX_GROUP+1)
	for g := Group(0); g <= MAX_GROUP; g++ {
		_Builders[g] = &builder{group: uint64(g)}
	}
}
func bitWidth(max int64) int {
	return utf8.RuneCountInString(strconv.FormatInt(max, 2))
}

// New a SUID with the given group. If group is not given, it will use the default group 0.
// The max group value is 7
func New(group ...Group) SUID {
	var g Group
	if len(group) > 0 {
		g = group[0]
	}
	b, ok := _Builders[g]
	if !ok {
		panic(fmt.Sprintf("[SUID] Invalid input group: %d", g))
	}
	return b.create()
}

// Parse creates a SUID from a suid string.
func Parse(str string) (s SUID, err error) {
	err = s.decodeFrom([]byte(str))
	return s, err
}

// String returns the base32-encoded string representation of the SUID.
func (s SUID) String() string {
	bytes := make([]byte, 13)
	s.encodeInto(bytes)
	return string(bytes)
}

// HostID returns the host ID of the SUID.
func (s SUID) HostID() uint8 {
	return uint8(s)
}

// Seq returns the sequence number of the SUID.
func (s SUID) Seq() int64 {
	return (int64(s) >> _WID_HOST) & MAX_SEQ
}

// Time returns the timestamp in seconds of the SUID.
func (s SUID) Time() int64 {
	return int64(s) >> (_WID_SEQ + _WID_HOST) & MAX_TIME
}

// Group returns the group of the SUID.
func (s SUID) Group() Group {
	return Group(uint64(s)>>(_WID_TIME+_WID_SEQ+_WID_HOST)) & MAX_GROUP
}

// Verify the SUID is valid or not.
func (s SUID) Verify() bool {
	return s.Group() <= MAX_GROUP && s.Seq() <= MAX_SEQ && s.Time() <= MAX_TIME && s.Time() > 1745400000 // 2025-04-23 17:20:00
}

// Get the description of the SUID.
func (s SUID) Description() string {
	return fmt.Sprintf("Group:%d, Time:%v, Seq:%d, Host:%d", s.Group(), time.Unix(s.Time(), 0), s.Seq(), s.HostID())
}

// MarshalJSON implements the json.Marshaler interface.
func (s SUID) MarshalJSON() ([]byte, error) {
	result := make([]byte, 15)
	result[0] = '"'
	s.encodeInto(result[1:])
	result[14] = '"'
	return result, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *SUID) UnmarshalJSON(data []byte) error {
	if len(data) != 15 || data[0] != '"' || data[14] != '"' {
		return fmt.Errorf("invalid suid json string")
	}
	return s.UnmarshalText(data[1:14])
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s SUID) MarshalText() ([]byte, error) {
	bytes := make([]byte, 13)
	s.encodeInto(bytes)
	return bytes, nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *SUID) UnmarshalText(data []byte) error {
	return s.decodeFrom(data)
}

// custom base32 encoding
func (s SUID) encodeInto(bytes []byte) {
	for j := 12; j >= 0; j-- {
		bytes[j] = coder.ENCODE[s&0x1f]
		s >>= 5
	}
}

// decodeFrom decodes a base32-encoded string to a uuint64 value.
func (s *SUID) decodeFrom(data []byte) error {
	if len(data) != 13 {
		return fmt.Errorf("invalid suid string length")
	}
	var value uint64
	for i := range 13 {
		idx, ok := coder.DECODE[data[i]]
		if !ok {
			return fmt.Errorf("invalid character in suid string: %c", data[i])
		}
		value = (value << 5) | uint64(idx)
	}
	*s = SUID(value)
	return nil
}

type builder struct {
	seq        int64
	lastSecond int64
	seqCount   int64
	group      uint64
	mx         sync.Mutex
}

// create creates a new SUID for the given group.
func (b *builder) create() SUID {
	b.mx.Lock()
	defer b.mx.Unlock()
	thisSecond := time.Now().Unix()
	if thisSecond < b.lastSecond { // clock moved backwards
		panic("[SUID] Fatal Error!!! Host Clock moved backwards!")
	}
	b.seq++
	if b.seq > MAX_SEQ { // overflow
		b.seq = 0
	}
	if thisSecond == b.lastSecond {
		b.seqCount++
		if b.seqCount > MAX_SEQ {
			b.seqCount = 0
			thisSecond = b.lastSecond + 1
			b.lastSecond = thisSecond
			dur := time.Until(time.Unix(thisSecond, 0))
			if dur > 0 {
				fmt.Printf("[SUID][WARN] Force wait(%s) to next second.\n", dur)
				time.Sleep(dur)
			}
		}
	} else {
		b.seqCount = 0
		b.lastSecond = thisSecond
	}
	return SUID(b.group<<(_WID_TIME+_WID_SEQ+_WID_HOST) | uint64(thisSecond)<<(_WID_SEQ+_WID_HOST) | uint64(b.seq)<<_WID_HOST | uint64(coder.HOSTID))
}
