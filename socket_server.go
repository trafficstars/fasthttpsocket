package fasthttpsocket

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/trafficstars/fasthttp"
	"github.com/trafficstars/spinlock"
)

type SocketServer struct {
	spinlock.Locker

	Logger         Logger
	NewEncoderFunc NewEncoderFunc
	NewDecoderFunc NewDecoderFunc
	DataModel      dataModel
	Family         Family
	Address        string

	HandleRequester       HandleRequester
	UnixSocketPermissions os.FileMode
}

func NewSocketServer(handleRequester HandleRequester, cfg Config) (*SocketServer, error) {
	var err error
	sock := &SocketServer{
		Logger:                cfg.Logger,
		HandleRequester:       handleRequester,
		UnixSocketPermissions: cfg.UnixSocketPermissions,
	}
	sock.NewEncoderFunc, sock.NewDecoderFunc, sock.DataModel, sock.Family, sock.Address, err = parseConfig(&cfg)
	if err != nil {
		return nil, err
	}
	return sock, err
}

func (sock *SocketServer) Start() error {
	if sock.Family == FamilyUnix {
		os.Remove(sock.Address)
	}
	accepter, err := net.Listen(sock.Family.String(), sock.Address)
	if err != nil {
		return fmt.Errorf(`[fasthttp-socket] Cannot bind "%v:%v"\n`, sock.Family, sock.Address)
	}
	if sock.Family == FamilyUnix {
		if err := os.Chmod(sock.Address, sock.UnixSocketPermissions); err != nil {
			return fmt.Errorf(`[fasthttp-socket] Cannot change permission on socket "%v" to "%v"`, sock.Address, sock.UnixSocketPermissions)
		}
	}

	// Starting
	logger := sock.Logger

	go func() {
		for {
			conn, err := accepter.Accept()
			if err != nil {
				logger.Errorf("[fasthttp-socket] got error: %v\n", err)
				continue
			}

			go func() {
				logger.Print(`[fasthttp-socket-handler] opened connection`)
				sock.handleSocketConnection(conn)
				logger.Print(`[fasthttp-socket-handler] closed connection`)
			}()
		}
	}()
	logger.Print("[fasthttp-socket] Started to listen ", sock.Address, " (", sock.UnixSocketPermissions, ")")

	return nil
}

func (sock *SocketServer) handleSocketConnection(conn net.Conn) {
	msg := NewMessanger(conn)

	encoder := sock.NewEncoderFunc(msg)
	decoder := sock.NewDecoderFunc(msg)

	logger := sock.Logger

	modelCodec := sock.DataModel.GetServerCodec()

	request := modelCodec.GetRequest()
	response := modelCodec.GetResponse()

	requestCtx := fasthttp.RequestCtx{}

	for {
		err := decoder.Decode(request)
		if err != nil {
			netErr, _ := err.(*net.OpError)
			switch {
			case err == io.EOF, netErr != nil && netErr.Err == io.EOF:
			default:
				logger.Errorf(`[fasthttp-socket-handler] got error (and closing): %v\n`, err)
			}
			break
		}

		err = modelCodec.Decode(&requestCtx, request)
		if err != nil {
			logger.Errorf(`[fasthttp-socket-handler] unable parse the request: %v\n`, err)
			break
		}

		err = sock.HandleRequester.HandleRequest(&requestCtx)
		if err != nil {
			logger.Errorf(`[fasthttp-socket-handler] unable process the request: %v\n`, err)
			break
		}

		err = modelCodec.Encode(response, &requestCtx)
		if err != nil {
			logger.Errorf(`[fasthttp-socket-handler] unable convert the response: %v\n`, err)
			break
		}

		err = encoder.Encode(response)
		if err != nil {
			logger.Errorf(`[fasthttp-socket-handler] unable to send a message: %v\n`, err)
			break
		}
	}

	request.Release()
	response.Release()

	_ = conn.Close()
}

func (sock *SocketServer) Stop() error {
	return ErrNotImplemented
}
