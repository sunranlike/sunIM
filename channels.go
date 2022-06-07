package sun

import (
	"sync"

	"github.com/sunrnalike/sun/logger"
)

// ChannelMap ChannelMap接口,这个是管理channel的一个channelmap接口,他倾向于管理channel,是server side的
type ChannelMap interface {
	Add(channel Channel)
	Remove(id string)
	Get(id string) (channel Channel, ok bool)
	All() []Channel
}

// ChannelsImpl ChannelMap 这个实现了channelmap接口的方法.可以作为ChannelMap的合法值
//ChannelsImpl是一个命名规范,他实现了Channels接口就可以后面价格Impl
type ChannelsImpl struct {
	// TODO: Optimization point
	channels *sync.Map
}

// NewChannels NewChannels
func NewChannels(num int) ChannelMap {
	return &ChannelsImpl{
		channels: new(sync.Map),
	}
}

//以下都是通过sync.Map这个并发安全的channel进行操作

// Add addChannel,
func (ch *ChannelsImpl) Add(channel Channel) {
	if channel.ID() == "" {
		logger.WithFields(logger.Fields{
			"module": "ChannelsImpl",
		}).Error("channel id is required")
	}
	//就是利用底层的并发安全sync.map存储一个channel
	ch.channels.Store(channel.ID(), channel)
}

// Remove addChannel
func (ch *ChannelsImpl) Remove(id string) {
	ch.channels.Delete(id)
}

// Get
func (ch *ChannelsImpl) Get(id string) (Channel, bool) {
	if id == "" {
		logger.WithFields(logger.Fields{
			"module": "ChannelsImpl",
		}).Error("channel id is required")
	}

	val, ok := ch.channels.Load(id)
	if !ok {
		return nil, false
	}
	return val.(Channel), true
}

// All return channels
func (ch *ChannelsImpl) All() []Channel {
	arr := make([]Channel, 0)
	ch.channels.Range(func(key, val interface{}) bool {
		arr = append(arr, val.(Channel))
		return true
	})
	return arr
}
