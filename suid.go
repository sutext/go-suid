package suid

import (
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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

const (
	MAX_SEQ   int64 = 0x7ffff     // 19 bits
	MAX_TIME  int64 = 0x3ffffffff // 34 bits
	MAX_GROUP uint8 = 0x7         // 3 bits
)

var (
	_WID_SEQ  = bitWidth(MAX_SEQ)  //SEQ bits width
	_WID_HOST = 8                  //HOST bits width
	_WID_TIME = bitWidth(MAX_TIME) //TIME bits width
	_HOST_ID  = getHostID()
	_ENCODE   = "abcdefghijklmnopqrstuvwxyz123456" // custom base32 encoding map
	_DECODE   = make(map[byte]byte, len(_ENCODE))  // custom base32 decoding map
	_Builders = sync.Map{}
)

func init() {
	for i := 0; i < len(_ENCODE); i++ {
		_DECODE[_ENCODE[i]] = byte(i)
	}
}
func bitWidth(max int64) int {
	return utf8.RuneCountInString(strconv.FormatInt(max, 2))
}

// New a SUID with the given group. If group is not given, it will use the default group 0.
// The max group value is 7
func New(group ...uint8) SUID {
	switch len(group) {
	case 0:
		return getBuilder(0).create()
	case 1:
		return getBuilder(group[0]).create()
	default:
		panic("[SUID New] Invalid parameter count.")
	}
}

// Get current Host ID.
func HostID() uint8 {
	return _HOST_ID
}

// FromInteger creates a SUID from an uint64 value.
func FromInteger(value uint64) SUID {
	return SUID(value)
}

// FromString creates a SUID from a suid string.
func FromString(str string) (s SUID, err error) {
	return decodeSUID([]byte(str))
}

// String returns the base32-encoded string representation of the SUID.
func (s SUID) String() string {
	bytes := make([]byte, 13)
	s.encodeInto(bytes)
	return string(bytes)
}

// Int returns the uint64 value of the SUID.
func (s SUID) Integer() uint64 {
	return uint64(s)
}

// Host returns the host ID of the SUID.
func (s SUID) Host() uint8 {
	return uint8(s)
}

// Seq returns the sequence number of the SUID.
func (s SUID) Seq() int64 {
	return (int64(s) >> _WID_HOST) & MAX_SEQ
}

// Time returns the timestamp of the SUID.
func (s SUID) Time() int64 {
	return int64(s) >> (_WID_SEQ + _WID_HOST) & MAX_TIME
}

// Group returns the group of the SUID.
func (s SUID) Group() uint8 {
	return uint8(uint64(s)>>(_WID_TIME+_WID_SEQ+_WID_HOST)) & MAX_GROUP
}

// Verify the SUID is valid or not.
func (s SUID) Verify() bool {
	return s.Group() <= MAX_GROUP && s.Seq() <= MAX_SEQ && s.Time() <= MAX_TIME && s.Time() > 1745400000 // 2025-04-23 17:20:00
}

// Get the description of the SUID.
func (s SUID) Description() string {
	return fmt.Sprintf("Group:%d, Time:%v, Seq:%d, Host:%d", s.Group(), time.Unix(s.Time(), 0), s.Seq(), s.Host())
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
	i, err := decodeSUID(data)
	if err != nil {
		return err
	}
	*s = i
	return nil
}

// custom base32 encoding
func (s SUID) encodeInto(bytes []byte) {
	for j := 12; j >= 0; j-- {
		bytes[j] = _ENCODE[s&0x1f]
		s >>= 5
	}
}

// decodeSUID decodes a base32-encoded string to a uuint64 value.
func decodeSUID(data []byte) (SUID, error) {
	if len(data) != 13 {
		return 0, fmt.Errorf("invalid suid string length")
	}
	var value uint64
	for i := range 13 {
		idx, ok := _DECODE[data[i]]
		if !ok {
			return 0, fmt.Errorf("invalid character in suid string: %c", data[i])
		}
		value = (value << 5) | uint64(idx)
	}
	return SUID(value), nil
}

type builder struct {
	seq      int64
	thisTime int64
	zeroTime int64
	seqCount int64
	group    uint64
	mx       sync.Mutex
}

// getBuilder returns the builder for the given group. If the builder does not exist, it will create a new one.
func getBuilder(g uint8) *builder {
	if g > MAX_GROUP {
		panic(fmt.Sprintf("[SUID] Invalid input group: %d", g))
	}
	b, _ := _Builders.LoadOrStore(g, newBuilder(g))
	return b.(*builder)
}

// newBuilder creates a new builder for the given group.
func newBuilder(group uint8) *builder {
	return &builder{seq: 0, thisTime: 0, zeroTime: time.Now().Unix(), seqCount: 0, group: uint64(group)}
}

// create creates a new SUID for the given group.
func (b *builder) create() SUID {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.thisTime = time.Now().Unix()
	if b.thisTime < b.zeroTime { // clock moved backwards
		panic("[SUID] Fatal Error!!! Host Clock moved backwards!")
	}
	b.seq++
	if b.seq > MAX_SEQ { // overflow
		b.seq = 0
	}
	if b.thisTime == b.zeroTime {
		b.seqCount++
		if b.seqCount > MAX_SEQ {
			b.seqCount = 0
			b.thisTime = b.zeroTime + 1
			b.zeroTime = b.thisTime
			dur := time.Until(time.Unix(b.thisTime, 0))
			if dur > 0 {
				fmt.Printf("[SUID][WARN] Force wait(%s) to next second.\n", dur)
				time.Sleep(dur)
			}
		}
	} else {
		b.seqCount = 0
		b.zeroTime = b.thisTime
	}
	return SUID(b.group<<(_WID_TIME+_WID_SEQ+_WID_HOST) | uint64(b.thisTime)<<(_WID_SEQ+_WID_HOST) | uint64(b.seq)<<_WID_HOST | uint64(_HOST_ID&0x7f))
}

func getHostID() uint8 {
	// read SUID_HOST_ID from environment firstly
	str := os.Getenv("SUID_HOST_ID")
	if str != "" {
		id, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return uint8(id)
		}
	}
	// read statefulset id from k8s environment secondly
	name := os.Getenv("POD_NAME")
	if name == "" {
		name, _ = os.Hostname()
	}
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		str := parts[len(parts)-1]
		id, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return uint8(id)
		}
	}
	// read local ip address thirdly
	if ip, err := getLocalIP(); err == nil {
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			last, err := strconv.ParseInt(parts[3], 10, 64)
			if err == nil {
				return uint8(last)
			}
		}
	}
	return uint8(rand.Int())
}

// getLocalIP retrieves the non-loopback local IPv4 address of the machine.
func getLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		// Check if the interface is up and not a loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no network interface found")
}
