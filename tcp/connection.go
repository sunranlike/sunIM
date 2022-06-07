package tcp

import (
	"github.com/sunrnalike/sun"
	"io"
	"net"

	"github.com/sunrnalike/sun/wire/endian"
)

// Frame Frame
type Frame struct {
	OpCode  sun.OpCode
	Payload []byte
}

//这些方法都是为了实现Frame接口

// SetOpCode SetOpCode
func (f *Frame) SetOpCode(code sun.OpCode) {
	f.OpCode = code
}

// GetOpCode GetOpCode
func (f *Frame) GetOpCode() sun.OpCode {
	return f.OpCode
}

// SetPayload SetPayload
func (f *Frame) SetPayload(payload []byte) {
	f.Payload = payload
}

// GetPayload GetPayload
func (f *Frame) GetPayload() []byte {
	return f.Payload
}

// TcpConn 显式嵌入net.Conn,在构造函数NewConn就是封装了一下net.Conn
type TcpConn struct {
	net.Conn
}

// NewConn NewConn
func NewConn(conn net.Conn) *TcpConn {
	return &TcpConn{
		Conn: conn,
	}
}

// ReadFrame 底层调用自己写的工具
func (c *TcpConn) ReadFrame() (sun.Frame, error) {
	opcode, err := endian.ReadUint8(c.Conn)
	if err != nil {
		return nil, err
	}
	payload, err := endian.ReadBytes(c.Conn)
	if err != nil {
		return nil, err
	}
	return &Frame{
		OpCode:  sun.OpCode(opcode),
		Payload: payload,
	}, nil
}

func (c *TcpConn) WriteFrame(code sun.OpCode, payload []byte) error {
	return WriteFrame(c.Conn, code, payload)
}

// Flush Flush
func (c *TcpConn) Flush() error {
	return nil
}

// WriteFrame write a frame to w
func WriteFrame(w io.Writer, code sun.OpCode, payload []byte) error {
	if err := endian.WriteUint8(w, uint8(code)); err != nil {
		return err
	}
	if err := endian.WriteBytes(w, payload); err != nil {
		return err
	}
	return nil
}
