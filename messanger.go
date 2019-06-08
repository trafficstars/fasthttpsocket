package fasthttpsocket

import (
	"fmt"
	"io"
	"net"
)

type Messanger interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}

func NewMessanger(conn io.ReadWriter) Messanger {
	var messanger Messanger
	switch conn := conn.(type) {
	case *net.UDPConn:
		messanger = &UDPMessanger{conn}
	case *net.UnixConn:
		messanger = &UnixMessanger{conn}
	default:
		panic(fmt.Errorf(`unknown type: %T`, conn))
	}
	return messanger
}

type UnixMessanger struct {
	*net.UnixConn
}

func (msg *UnixMessanger) Read(b []byte) (int, error) {
	n, _, _, _, err := msg.ReadMsgUnix(b, nil)
	return n, err
}

func (msg *UnixMessanger) Write(b []byte) (int, error) {
	n, _, err := msg.WriteMsgUnix(b, nil, nil)
	return n, err
}

type UDPMessanger struct {
	*net.UDPConn
}

func (msg *UDPMessanger) Read(b []byte) (int, error) {
	n, _, _, _, err := msg.ReadMsgUDP(b, nil)
	return n, err
}

func (msg *UDPMessanger) Write(b []byte) (int, error) {
	n, _, err := msg.WriteMsgUDP(b, nil, nil)
	return n, err
}
