package suid

import (
	"database/sql/driver"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// Snowflake Unique Identifier SUID is a 64-bit integer.
// The data structure is symbol(1)-time(33)-seq(22)-host(8)
//   - The host_id will read `SUID_HOST_ID` or `POD_NAME` from environment, and pick last part ( separator "-" ) or local ip address.
//   - If run in kubenetes, recommend to use `StatefulSet` to provide a unique host_id.
//   - Otherwise, you must set `SUID_HOST_ID` manually to provide a unique host_id,or keep unique ip.
//   - The SUID will work well before `2242-03-16 12:56:31 +0000 UTC`
//   - The SUID will keep threaded-safe and safe for concurrent access.
//   - The SUID will keep unique across multiple machines if host_id is unique.
//   - The SUID will keep unique on a single machine within one second.
//   - The SUID can generate up to `4194303` unique IDs per second on a single machine. If more IDs are needed, consider using multiple machines with different host_ids.
//   - The SUID is k-sortable.
//   - The SUID is encoded to a 13-character string using a custom base32 encoding (a-z, 1-6).
//   - The SUID can be used as primary key in database, and implement gorm's DataTypeInterface.
//   - The SUID implements json.Marshaler, json.Unmarshaler, encoding.TextMarshaler, encoding.TextUnmarshaler, driver.Valuer, sql.Scanner interfaces.
//   - The SUID can be used in distributed system without coordination.
//   - The SUID is not a cryptographically secure random number.
type SUID struct {
	value int64
}

// HostID returns the host ID of the current machine.
//
//   - The unique host ID is important to ensure the uniqueness of SUIDs across multiple machines.
//
//   - You can set the host ID by setting the `SUID_HOST_ID` environment variable or using a `StatefulSet` in Kubernetes.
//
//   - If not set, the host ID will be derived from the hostname or local IP address, or randomly generated as a last resort.
//
//   - Some times, a unique host ID is usefull in your distributed systems.
func HostID() int64 {
	return _HOST_ID
}

const (
	MAX_HOST int64 = 0xff        // 8 bits
	MAX_SEQ  int64 = 0x3fffff    // 22 bits
	MAX_TIME int64 = 0x1ffffffff // 33 bits
)

var (
	_WID_SEQ  = bitWidth(MAX_SEQ)  //SEQ bits width
	_WID_HOST = bitWidth(MAX_HOST) //HOST bits width
	_HOST_ID  = getHostID()
	_SEQ      = atomic.Int64{}
	_ENCODE   = "abcdefghijklmnopqrstuvwxyz123456" // custom base32 encoding map
	_DECODE   = make(map[byte]byte, len(_ENCODE))  // custom base32 decoding map
)

func init() {
	for i := 0; i < len(_ENCODE); i++ {
		_DECODE[_ENCODE[i]] = byte(i)
	}
}
func bitWidth(max int64) int64 {
	return int64(utf8.RuneCountInString(strconv.FormatInt(max, 2)))
}

// New generates a new SUID.
func New() SUID {
	seq := _SEQ.Add(1)
	seq = seq % MAX_SEQ
	thisTime := time.Now().UTC().Unix()
	return SUID{thisTime<<(_WID_SEQ+_WID_HOST) | seq<<_WID_HOST | _HOST_ID}
}

// FromInt creates a SUID from an int64 value.
func FromInteger(value int64) SUID {
	return SUID{value}
}

// FromHex creates a SUID from a hex string.
func FromString(str string) (SUID, error) {
	i, err := decode([]byte(str))
	if err != nil {
		return SUID{}, err
	}
	return SUID{i}, nil
}

// Seq returns the sequence number of the SUID.
func (s SUID) Seq() int64 {
	return s.value >> _WID_HOST & MAX_SEQ
}

// Host returns the host ID of the SUID.
func (s SUID) Host() int64 {
	return s.value & MAX_HOST
}

// Time returns the timestamp of the SUID.
func (s SUID) Time() int64 {
	return s.value >> (_WID_SEQ + _WID_HOST) & MAX_TIME
}

// Verify the SUID is valid or not.
func (s SUID) Verify() bool {
	return s.Time() > 1745400000 // 2025-04-23 17:20:00
}

// String returns the base32-encoded string representation of the SUID.
func (s SUID) String() string {
	return string(encode(s.value))
}

// Int returns the int64 value of the SUID.
func (s SUID) Integer() int64 {
	return s.value
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

// getHostID returns the host ID of the current machine.
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
	ip, err := getLocalIP()
	if err == nil {
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
