package suid

import (
	"database/sql/driver"
	"fmt"
	"math/rand"
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
type SUID struct {
	value int64
}

const (
	MAX_HOST  int64 = 0x7f        // 7 bits
	MAX_SEQ   int64 = 0x7ffff     // 19 bits
	MAX_TIME  int64 = 0x3ffffffff // 34 bits
	MAX_GROUP int64 = 0x7         // 3 bits
)

var (
	_WID_SEQ     = bitWidth(MAX_SEQ)  //SEQ bits width
	_WID_HOST    = bitWidth(MAX_HOST) //HOST bits width
	_WID_TIME    = bitWidth(MAX_TIME) //TIME bits width
	_HOST_ID     = getHostID()
	_ENCODE      = "abcdefghijklmnopqrstuvwxyz123456" // custom base32 encoding map
	_DECODE      = make(map[byte]byte, len(_ENCODE))  // custom base32 decoding map
	_Builders    = make(map[int64]*builder)
	_BuilderLock = sync.Mutex{}
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
func New(group ...int64) SUID {
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
func HostID() int64 {
	return _HOST_ID
}

// FromInteger creates a SUID from an int64 value.
func FromInteger(value int64) SUID {
	return SUID{value}
}

// FromString creates a SUID from a suid string.
func FromString(str string) (SUID, error) {
	i, err := decode([]byte(str))
	if err != nil {
		return SUID{}, err
	}
	return SUID{i}, nil
}

// String returns the base32-encoded string representation of the SUID.
func (s SUID) String() string {
	return string(encode(s.value))
}

// Int returns the int64 value of the SUID.
func (s SUID) Integer() int64 {
	return s.value
}

// Host returns the host ID of the SUID.
func (s SUID) Host() int64 {
	return s.value & MAX_HOST
}

// Seq returns the sequence number of the SUID.
func (s SUID) Seq() int64 {
	return (s.value >> _WID_HOST) & MAX_SEQ
}

// Time returns the timestamp of the SUID.
func (s SUID) Time() int64 {
	return s.value >> (_WID_SEQ + _WID_HOST) & MAX_TIME
}

// Group returns the group of the SUID.
func (s SUID) Group() int64 {
	return (s.value >> (_WID_TIME + _WID_SEQ + _WID_HOST)) & MAX_GROUP
}

// Verify the SUID is valid or not.
func (s SUID) Verify() bool {
	return s.Group() >= 0 && s.Group() <= MAX_GROUP && s.Seq() >= 0 && s.Seq() <= MAX_SEQ && s.Time() >= 0 && s.Time() <= MAX_TIME && s.Time() > 1745400000 // 2025-04-23 17:20:00
}

// Get the description of the SUID.
func (s SUID) Description() string {
	return fmt.Sprintf("Group:%d, Time:%d, Seq:%d, Host:%d", s.Group(), s.Time(), s.Seq(), s.Host())
}

// MarshalJSON implements the json.Marshaler interface.
func (s SUID) MarshalJSON() ([]byte, error) {
	result := make([]byte, 15)
	result[0] = '"'
	copy(result[1:], encode(s.value))
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
	return encode(s.value), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *SUID) UnmarshalText(data []byte) error {
	i, err := decode(data)
	if err != nil {
		return err
	}
	s.value = i
	return nil
}

// Value implements the driver.Valuer interface.
func (s SUID) Value() (driver.Value, error) {
	return s.value, nil
}

// Scan implements the sql.Scanner interface.
func (s *SUID) Scan(value any) error {
	switch v := value.(type) {
	case int64:
		s.value = v
		return nil
	default:
		return fmt.Errorf("unsupported type for SUID: %T", value)
	}
}

// GormDataType implements the gorm.DataTypeInterface interface.
func (SUID) GormDataType() string {
	return "bigint"
}

// custom base32 encoding
func encode(i int64) []byte {
	bytes := make([]byte, 13)
	for j := 12; j >= 0; j-- {
		bytes[j] = _ENCODE[i&0x1f]
		i >>= 5
	}
	return bytes
}

// decode decodes a base32-encoded string to a uint64 value.
func decode(data []byte) (int64, error) {
	if len(data) != 13 {
		return 0, fmt.Errorf("invalid suid string length")
	}
	var value int64
	for i := range 13 {
		idx, ok := _DECODE[data[i]]
		if !ok {
			return 0, fmt.Errorf("invalid character in suid string: %c", data[i])
		}
		value = (value << 5) | int64(idx)
	}
	return value, nil
}

type builder struct {
	seq      int64
	thisTime int64
	zeroTime int64
	seqCount int64
	group    int64
	mx       sync.Mutex
}

// getBuilder returns the builder for the given group. If the builder does not exist, it will create a new one.
func getBuilder(g int64) *builder {
	_BuilderLock.Lock()
	defer _BuilderLock.Unlock()
	if g < 0 || g > MAX_GROUP {
		panic(fmt.Sprintf("[SUID] Invalid input group: %d", g))
	}
	b, ok := _Builders[g]
	if !ok {
		b = newBuilder(g)
		_Builders[g] = b
	}
	return b
}

// newBuilder creates a new builder for the given group.
func newBuilder(group int64) *builder {
	return &builder{seq: 0, thisTime: 0, zeroTime: time.Now().Unix(), seqCount: 0, group: group}
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
	return SUID{b.group<<(_WID_TIME+_WID_SEQ+_WID_HOST) | b.thisTime<<(_WID_SEQ+_WID_HOST) | b.seq<<_WID_HOST | _HOST_ID}
}

func getHostID() int64 {
	// read SUID_HOST_ID from environment firstly
	str := os.Getenv("SUID_HOST_ID")
	if str != "" {
		id, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return id & MAX_HOST
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
			return id & MAX_HOST
		}
	}
	// read local ip address thirdly
	if ip, err := getLocalIP(); err == nil {
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			last, err := strconv.ParseInt(parts[3], 10, 64)
			if err == nil {
				return last & MAX_HOST
			}
		}
	}
	// finally, random a host id
	return rand.Int63() & MAX_HOST
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
