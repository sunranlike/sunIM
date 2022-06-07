package container

import (
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/wire/pkt"
	"hash/crc32"
)

// HashCode generated a hash code
func HashCode(key string) int {
	hash32 := crc32.NewIEEE()
	hash32.Write([]byte(key))
	return int(hash32.Sum32())
}

// Selector is used to select a Service
type Selector interface {
	Lookup(*pkt.Header, []sun.Service) string
}
