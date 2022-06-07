package sun

import (
	"errors"
	"github.com/sunrnalike/sun/wire/pkt"
)

// ErrNil
var ErrSessionNil = errors.New("err:session nil")

// SessionStorage defined a session storage which provides based functions as save,delete,find a session
type SessionStorage interface {
	// Add a session
	Add(session *pkt.Session) error
	// Delete a session
	Delete(account string, channelId string) error
	// Get session by channelId
	Get(channelId string) (*pkt.Session, error)
	//批量读取位置信息，主要用于群聊时读取群成员定位信息。
	GetLocations(account ...string) ([]*Location, error)
	//
	GetLocation(account string, device string) (*Location, error)
}
