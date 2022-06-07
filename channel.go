package sun

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sunrnalike/sun/logger"
)

// ChannelImpl is a websocket implement of channel
//这个结构体实现了channel接口的方法,被用作new方法的返回值
type ChannelImpl struct {
	sync.Mutex
	id string
	Conn
	writechan chan []byte
	once      sync.Once
	writeWait time.Duration
	readwait  time.Duration
	closed    *Event
}

// NewChannel 构造函数,channel是干嘛的?他不是goroutine的channel,而是代表一个频道的意思,存放的是连接.
func NewChannel(id string, conn Conn) Channel {
	log := logger.WithFields(logger.Fields{
		"module": "channel",
		"id":     id,
	})
	ch := &ChannelImpl{
		id:        id,
		Conn:      conn,
		writechan: make(chan []byte, 5),
		closed:    NewEvent(),
		writeWait: DefaultWriteWait, //default value
		readwait:  DefaultReadWait,
	}
	go func() { //单独开启一个goroutine,去执行writeloop
		err := ch.writeloop()
		if err != nil {
			log.Info(err)
		}
	}()
	return ch
}

//writeloop的实现
func (ch *ChannelImpl) writeloop() error {
	for { //for+select+chan无限循环,case payload就读取数据,case Done就退出,不case就下一次for循环
		select {
		case payload := <-ch.writechan:
			err := ch.WriteFrame(OpBinary, payload)
			if err != nil {
				return err
			}
			chanlen := len(ch.writechan)
			for i := 0; i < chanlen; i++ {
				payload = <-ch.writechan
				err := ch.WriteFrame(OpBinary, payload)
				if err != nil {
					return err
				}
			}
			err = ch.Conn.Flush()
			if err != nil {
				return err
			}
		case <-ch.closed.Done():
			return nil
		}
	}
}

// ID id
func (ch *ChannelImpl) ID() string { return ch.id }

// Send 异步写数据
func (ch *ChannelImpl) Push(payload []byte) error {
	if ch.closed.HasFired() {
		return fmt.Errorf("channel %s has closed", ch.id)
	}
	// 异步写
	ch.writechan <- payload
	return nil
}

// overwrite Conn
func (ch *ChannelImpl) WriteFrame(code OpCode, payload []byte) error {
	_ = ch.Conn.SetWriteDeadline(time.Now().Add(ch.writeWait))
	return ch.Conn.WriteFrame(code, payload)
}

// Close 关闭连接
func (ch *ChannelImpl) Close() error {
	ch.once.Do(func() {
		close(ch.writechan)
		ch.closed.Fire()
	})
	return nil
}

// SetWriteWait 设置写超时
func (ch *ChannelImpl) SetWriteWait(writeWait time.Duration) {
	if writeWait == 0 {
		return
	}
	ch.writeWait = writeWait
}

func (ch *ChannelImpl) SetReadWait(readwait time.Duration) {
	if readwait == 0 {
		return
	}
	ch.writeWait = readwait
}

func (ch *ChannelImpl) Readloop(lst MessageListener) error {
	ch.Lock()
	defer ch.Unlock()
	log := logger.WithFields(logger.Fields{
		"struct": "ChannelImpl",
		"func":   "Readloop",
		"id":     ch.id,
	})
	for {
		_ = ch.SetReadDeadline(time.Now().Add(ch.readwait))

		frame, err := ch.ReadFrame()
		if err != nil {
			return err
		}
		if frame.GetOpCode() == OpClose {
			return errors.New("remote side close the channel")
		}
		if frame.GetOpCode() == OpPing {
			log.Trace("recv a ping; resp with a pong")
			_ = ch.WriteFrame(OpPong, nil)
			continue
		}
		payload := frame.GetPayload()
		if len(payload) == 0 {
			continue
		}
		// TODO: Optimization point
		go lst.Receive(ch, payload)
	}
}
