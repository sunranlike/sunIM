package container

import (
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/wire/pkt"
)

// HashSelector HashSelector
type HashSelector struct {
}

// Lookup a server
func (s *HashSelector) Lookup(header *pkt.Header, srvs []sun.Service) string {
	ll := len(srvs)
	code := HashCode(header.ChannelId)
	return srvs[code%ll].ServiceID()
}
