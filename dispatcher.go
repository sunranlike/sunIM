package sun

import "github.com/sunrnalike/sun/wire/pkt"

// Dispather defined a component how a message be dispatched to gateway
type Dispatcher interface {
	Push(gateway string, channels []string, p *pkt.LogicPkt) error
}
