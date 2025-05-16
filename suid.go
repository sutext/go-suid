package suid

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/rand"
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

// FromInt creates a SUID from an int64 value.
func FromInt(value int64) SUID {
	return SUID{value}
}

// FromStr creates a SUID from a string.
func FromStr(str string) (SUID, error) {
	parsed, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return SUID{}, err
	}
	return SUID{parsed}, nil
}

// FromHex creates a SUID from a hex string.
func FromHex(str string) (SUID, error) {
	parsed, err := strconv.ParseInt(str, 16, 64)
	if err != nil {
		return SUID{}, err
	}
	return SUID{parsed}, nil
}

// Int returns the int64 value of the SUID.
func (s SUID) Int() int64 {
	return s.value
}

// Hex returns the hex string representation of the SUID.
func (s SUID) Hex() string {
	return s.String(16)
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

// String returns the string representation of the SUID with the given base.
// If base is not given, it will use base 10.
func (s SUID) String(base ...int) string {
	switch len(base) {
	case 0:
		return strconv.FormatInt(s.value, 10)
	case 1:
		return strconv.FormatInt(s.value, base[0])
	default:
		panic("[SUID String] Invalid parameter count.")
	}
}

// Verify the SUID is valid or not.
func (s SUID) Verify() bool {
	return s.Group() >= 0 && s.Group() <= MAX_GROUP && s.Seq() >= 0 && s.Seq() <= MAX_SEQ && s.Time() >= 0 && s.Time() <= MAX_TIME && s.Time() > 1745400000 // 2025-04-23 17:20:00
}

// Get the description of the SUID.
func (s SUID) Description() string {
	return fmt.Sprintf("Group:%d, Time:%d, Hex:%s", s.Group(), s.Time(), s.Hex())
}

// MarshalJSON implements the json.Marshaler interface.
func (s SUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *SUID) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &s.value)
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
func (s SUID) GormDataType() string {
	return "bigint"
}

const (
	MAX_HOST  int64 = 0x7f        // 7 bits
	MAX_SEQ   int64 = 0x7ffff     // 19 bits
	MAX_TIME  int64 = 0x3ffffffff // 34 bits
	MAX_GROUP int64 = 0x7         // 3 bits
)

var (
	_WID_SEQ     = utf8.RuneCountInString(strconv.FormatInt(MAX_SEQ, 2))  //SEQ bits width
	_WID_HOST    = utf8.RuneCountInString(strconv.FormatInt(MAX_HOST, 2)) //HOST bits width
	_WID_TIME    = utf8.RuneCountInString(strconv.FormatInt(MAX_TIME, 2)) //TIME bits width
	_HOST_ID     = getHostID()
	_Builders    = make(map[int64]*builder)
	_BuilderLock = sync.Mutex{}
)

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

// getHostID returns the host ID of the current machine.
// It will read `SUID_HOST_ID` or `POD_NAME` `HOSTNAME` from environment firstly, and pick last part ( separator "-" )
// If host id is not found, it will generate a random host id.
func getHostID() int64 {
	// read SUID_HOST_ID from environment firstly
	str := os.Getenv("SUID_HOST_ID")
	if str != "" {
		id, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return id & MAX_HOST
		}
	}
	// read hostname from k8s environment secondly
	name := os.Getenv("POD_NAME")
	if name == "" {
		name = os.Getenv("HOSTNAME")
	}
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		str := parts[len(parts)-1]
		id, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return id & MAX_HOST
		}
	}
	// generate random host ID
	return rand.Int63() & MAX_HOST
}
