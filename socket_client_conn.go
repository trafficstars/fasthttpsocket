package fasthttpsocket

import (
	"fmt"
	"io"
	"net"

	"github.com/trafficstars/fasthttp"
	"github.com/trafficstars/spinlock"
)

type SocketClientConn struct {
	spinlock.Locker

	*SocketClient

	net.Conn
	Messanger
	Encoder
	Decoder
	ModelCodec ClientCodec

	Request  TransmittableRequest
	Response TransmittableResponse
}

func newSocketClientConn(
	sock *SocketClient,
) (*SocketClientConn, error) {
	c := &SocketClientConn{SocketClient: sock}
	_ = c.Reconnect()

	c.Encoder = sock.NewEncoderFunc(c)
	c.Decoder = sock.NewDecoderFunc(c)

	c.ModelCodec = sock.DataModel.GetClientCodec()

	c.Request = c.ModelCodec.GetRequest()
	c.Response = c.ModelCodec.GetResponse()
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
	sock := c.SocketClient

	sock.LockDo(func() {
		connIdx := -1
		for idx, conn := range sock.clientConns {
			if conn == c {
				connIdx = idx
				break
			}
		}
		if connIdx == -1 {
			panic(`closing an unknown connection :(`)
		}
		newClientConns := sock.clientConns[:connIdx]
		if connIdx < len(sock.clientConns) {
			newClientConns = append(newClientConns, sock.clientConns[connIdx+1:]...)
		}
		sock.clientConns = newClientConns
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

func (c *SocketClientConn) sendAndReceive(ctx *fasthttp.RequestCtx) error {
	request := c.Request
	response := c.Response

	err := c.ModelCodec.Encode(request, ctx)
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

	err = c.ModelCodec.Decode(ctx, response)
	if err != nil {
		return err
	}

	return nil
}

func (c *SocketClientConn) SendAndReceive(ctx *fasthttp.RequestCtx) (ret error) {
	defer func() { // gob.Decoder panics sometimes
		if r := recover(); r != nil {
			ret = fmt.Errorf("panic: %v", r)
		}
	}()
	return c.sendAndReceive(ctx)
}
