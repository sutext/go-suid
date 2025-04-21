package suid

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

type SUID struct {
	value int64
}
type Plane int64

const (
	PlaneA Plane = 0x1000_0000_0000_0000
	PlaneB Plane = 0x2000_0000_0000_0000
	PlaneC Plane = 0x3000_0000_0000_0000
	PlaneD Plane = 0x4000_0000_0000_0000
	PlaneE Plane = 0x5000_0000_0000_0000
	PlaneF Plane = 0x6000_0000_0000_0000
	PlaneG Plane = 0x7000_0000_0000_0000
)

func (p Plane) String() string {
	switch p {
	case PlaneA:
		return "A"
	case PlaneB:
		return "B"
	case PlaneC:
		return "C"
	case PlaneD:
		return "D"
	case PlaneE:
		return "E"
	case PlaneF:
		return "F"
	case PlaneG:
		return "G"
	default:
		return "Unknown"
	}
}
func New(p Plane) SUID {
	return _builder.create(p)
}
func NewA() SUID {
	return _builder.create(PlaneA)
}
func (s SUID) Host() int64 {
	return s.value & mask_host
}
func (s SUID) Seq() int64 {
	return (s.value >> width_host) & mask_seq
}
func (s SUID) Time() int64 {
	return s.value >> (width_host + width_seq) & mask_time
}
func (s SUID) Plane() Plane {
	return Plane(s.value & int64(PlaneG))
}
func (s SUID) String() string {
	return strconv.FormatInt(s.value, 10)
}
func (s SUID) Verify() bool {
	return s.Time() > 1678204800
}
func (s SUID) Desc() string {
	return fmt.Sprintf("\nPlane:%s \nHost:%d \nSeq:%d \nTime:%s", s.Plane(), s.Host(), s.Seq(), time.Unix(s.Time(), 0).Format(time.DateTime))
}
func (s SUID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}
func (s *SUID) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return nil
	}
	s.value, _ = strconv.ParseInt(string(data[1:len(data)-1]), 10, 64)
	return nil
}

const (
	mask_seq  int64 = 0x3ffff
	mask_host int64 = 0xff
	mask_time int64 = 0x3ffffffff
)

var (
	width_seq  = utf8.RuneCountInString(strconv.FormatInt(mask_seq, 2))  //SEQ bits width
	width_host = utf8.RuneCountInString(strconv.FormatInt(mask_host, 2)) //HOST bits width
	// width_time = utf8.RuneCountInString(strconv.FormatInt(mask_time, 2)) //TIME bits width
	host_id  = getHostID()
	_builder = &builder{seq: 0, thisTime: 0, zeroTime: time.Now().Unix()}
)

type builder struct {
	seq      int64
	thisTime int64
	zeroTime int64
	mx       sync.Mutex
}

func (b *builder) create(p Plane) SUID {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.thisTime = time.Now().Unix()
	if b.thisTime < b.zeroTime { // clock moved backwards
		panic("[SUID] Fatal Error!!! Host Clock moved backwards!")
	}
	if b.thisTime == b.zeroTime {
		b.seq++
		if b.seq > mask_seq { // overflow
			b.thisTime = b.zeroTime + 1
			b.seq = 0
			b.zeroTime = b.thisTime
			dur := time.Until(time.Unix(b.thisTime, 0))
			if dur > 0 {
				fmt.Println("[SUID] Force wait(ms):", dur)
				time.Sleep(dur) // wait for next second
			}
		}
	} else {
		b.seq = 0
		b.zeroTime = b.thisTime
	}
	return SUID{int64(p) | b.thisTime<<(width_host+width_seq) | b.seq<<width_host | host_id}
}
func getHostID() int64 {
	str := os.Getenv("SUID_HOST_ID")
	if str != "" {
		id, _ := strconv.ParseInt(str, 10, 64)
		return id & mask_host
	}
	return rand.Int63() & mask_host
}
