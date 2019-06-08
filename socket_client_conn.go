package fasthttpsocket

import (
	"github.com/trafficstars/fasthttp"
	"github.com/trafficstars/spinlock"
	"io"
	"net"
)

type SocketClientConn struct {
	spinlock.Locker

	*Socket

	net.Conn
	Messanger
	Encoder
	Decoder
	ClientCodec

	Family  family
	Address string

	Request  TransmittableRequest
	Response TransmittableResponse
}

func newSocketClientConn(
	socket *Socket,
	newEncoderFunc NewEncoderFunc,
	newDecoderFunc NewDecoderFunc,
	dataModel dataModel,
	family family,
	address string,
) (*SocketClientConn, error) {
	c := &SocketClientConn{Socket: socket}
	c.Family = family
	c.Address = address
	_ = c.Reconnect()

	c.Encoder = newEncoderFunc(c)
	c.Decoder = newDecoderFunc(c)
	c.ClientCodec = dataModel.GetClientCodec()
	c.Request = c.ClientCodec.GetRequest()
	c.Response = c.ClientCodec.GetResponse()
	return c, nil
}

func (c *SocketClientConn) Reconnect() error {
	var err error
	if c.Conn != nil {
		c.Conn.Close()
	}
	c.Conn, err = net.Dial(c.Family.String(), c.Address)
	if err == nil {
		c.Messanger = NewMessanger(c.Conn)
	}
	return err
}

func (c *SocketClientConn) Close() {
	c.Socket.LockDo(func() {
		connIdx := -1
		for idx, conn := range c.Socket.clientConns {
			if conn == c {
				connIdx = idx
				break
			}
		}
		if connIdx == -1 {
			panic(`closing an unknown connection :(`)
		}
		newClientConns := c.Socket.clientConns[:connIdx]
		if connIdx < len(c.Socket.clientConns) {
			newClientConns = append(newClientConns, c.Socket.clientConns[connIdx+1:]...)
		}
		c.Socket.clientConns = newClientConns
	})

	_ = c.Conn.Close()
	c.Messanger = nil
}

func (c *SocketClientConn) Read(b []byte) (int, error) {
	if c.Messanger == nil {
		return 0, io.ErrClosedPipe
	}
	return c.Messanger.Read(b)
}

func (c *SocketClientConn) Write(b []byte) (int, error) {
	if c.Messanger == nil {
		return 0, io.ErrClosedPipe
	}
	return c.Messanger.Write(b)
}

func (c *SocketClientConn) release() {
	c.Unlock()
}

func (c *SocketClientConn) SendAndReceive(ctx *fasthttp.RequestCtx) error {
	request := c.Request
	response := c.Response

	err := c.ClientCodec.Encode(request, ctx)
	if err != nil {
		return err
	}

	err = c.Encoder.Encode(request)
	if err != nil {
		err2 := c.Reconnect()
		if err2 != nil {
			return err2
		}
		err = c.Encoder.Encode(request)
		if err != nil {
			return err
		}
	}

	err = c.Decoder.Decode(response)
	if err != nil {
		err2 := c.Reconnect()
		if err2 != nil {
			return err2
		}
		return err
	}

	err = c.ClientCodec.Decode(ctx, response)
	if err != nil {
		return err
	}

	return nil
}
