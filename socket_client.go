package fasthttpsocket

import (
	"github.com/pkg/errors"
	"github.com/trafficstars/fasthttp"
	"github.com/trafficstars/spinlock"
)

var (
	ErrBusy = errors.New(`[fasthttp-socket-client] all connections are busy`)
)

type SocketClient struct {
	spinlock.Locker

	Logger         Logger
	NewEncoderFunc NewEncoderFunc
	NewDecoderFunc NewDecoderFunc
	DataModel      dataModel
	Family         Family
	Address        string

	clientConnPointer   int
	clientConns         []*SocketClientConn
	requiredClientConns int
}

func NewSocketClient(cfg Config) (*SocketClient, error) {
	var err error
	sock := &SocketClient{
		Logger: cfg.Logger,
	}
	sock.NewEncoderFunc, sock.NewDecoderFunc, sock.DataModel, sock.Family, sock.Address, err = parseConfig(&cfg)
	if err != nil {
		return nil, err
	}
	return sock, err
}

func (sock *SocketClient) Start(connCount int) error {
	sock.requiredClientConns = connCount

	for sock.getClientConnCount() < sock.requiredClientConns {
		err := sock.addClientConn()
		if err != nil {
			return err
		}
	}

	return nil
}

func (sock *SocketClient) getClientConnCount() (r int) {
	sock.LockDo(func() {
		r = len(sock.clientConns)
	})
	return
}

func (sock *SocketClient) addClientConn() error {
	conn, err := newSocketClientConn(sock)
	if err != nil {
		return err
	}
	sock.LockDo(func() {
		sock.clientConns = append(sock.clientConns, conn)
	})
	return nil
}

func (sock *SocketClient) acquireClientConnection() (r *SocketClientConn) {
	sock.LockDo(func() {
		if int(sock.clientConnPointer) >= len(sock.clientConns) {
			sock.clientConnPointer = 0
		}
		oldIdx := sock.clientConnPointer
		for !sock.clientConns[sock.clientConnPointer].TryLock() {
			sock.clientConnPointer++
			if int(sock.clientConnPointer) >= len(sock.clientConns) {
				sock.clientConnPointer = 0
			}
			if sock.clientConnPointer == oldIdx { // all connections are busy, cannot lock/acquire any connection
				return
			}
		}
		r = sock.clientConns[sock.clientConnPointer]
		sock.clientConnPointer++
	})
	return
}

func (sock *SocketClient) SendAndReceive(ctx *fasthttp.RequestCtx) error {
	conn := sock.acquireClientConnection()
	if conn == nil {
		return ErrBusy
	}

	defer conn.release()

	err := conn.SendAndReceive(ctx)
	if err != nil {
		return err
	}

	return nil
}
