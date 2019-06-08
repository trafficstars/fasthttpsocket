package fasthttpsocket

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/trafficstars/fasthttp"
	"github.com/trafficstars/spinlock"
)

var (
	ErrUnknownFamily     = errors.New(`[fasthttp-socket] unknown family/transport`)
	ErrUnknownSerializer = errors.New(`[fasthttp-socket] unknown serializer`)
	ErrUnknownDataModel  = errors.New(`[fasthttp-socket] unknown data model`)
	ErrNotEnoughWords    = errors.New(`[fasthttp-socket] invalid address, expected syntax "datamodel:serializer:family:address", example "go/net/http:gob:unix:/run/myserver.sock"`)
	ErrNotImplemented    = errors.New(`[fasthttp-socket] not implemented, yet`)
	ErrBusy              = errors.New(`[fasthttp-socket-client] all connections are busy`)
)

type family int

const (
	familyUnix = iota
	familyUDP
	familyTCP
)

func (f family) String() string {
	switch f {
	case familyUnix:
		return `unix`
	case familyUDP:
		return `udp`
	case familyTCP:
		return `tcp`
	default:
		return ``
	}
}

type serializerType int

const (
	serializerTypeGob = iota
	serializerTypeJSON
)

type dataModel int

const (
	dataModelNetHttp = iota
)

func (dataModel dataModel) GetServerCodec() ServerCodec {
	switch dataModel {
	case dataModelNetHttp:
		return newServerCodecNetHttp()
	}
	return nil
}

func (dataModel dataModel) GetClientCodec() ClientCodec {
	switch dataModel {
	case dataModelNetHttp:
		return newClientCodecNetHttp()
	}
	return nil
}

type HandleRequester interface {
	HandleRequest(ctx *fasthttp.RequestCtx) error
}

type Encoder interface {
	Encode(e interface{}) error
}

type NewEncoderFunc func(w io.Writer) Encoder

type Decoder interface {
	Decode(e interface{}) error
}

type NewDecoderFunc func(r io.Reader) Decoder

type Socket struct {
	spinlock.Locker

	HandleRequester     HandleRequester
	Config              Config
	clientConnPointer   int
	clientConns         []*SocketClientConn
	requiredClientConns int
}

func NewSocket(handleRequester HandleRequester, cfg Config) *Socket {
	return &Socket{
		HandleRequester: handleRequester,
		Config:          cfg,
	}
}

func (sock *Socket) StartClient(connCount int) error {
	newEncoderFunc, newDecoderFunc, modelCodec, family, address, err := sock.parseConfig()
	if err != nil {
		return err
	}

	sock.requiredClientConns = connCount

	for sock.getClientConnCount() < sock.requiredClientConns {
		err := sock.addClientConn(newEncoderFunc, newDecoderFunc, modelCodec, family, address)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sock *Socket) getClientConnCount() (r int) {
	sock.LockDo(func() {
		r = len(sock.clientConns)
	})
	return
}

func (sock *Socket) addClientConn(
	newEncoderFunc NewEncoderFunc,
	newDecoderFunc NewDecoderFunc,
	dataModel dataModel,
	family family,
	address string,
) error {
	conn, err := newSocketClientConn(sock, newEncoderFunc, newDecoderFunc, dataModel, family, address)
	if err != nil {
		return err
	}
	sock.LockDo(func() {
		sock.clientConns = append(sock.clientConns, conn)
	})
	return nil
}

func (sock *Socket) acquireClientConnection() (r *SocketClientConn) {
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

func (sock *Socket) SendAndReceive(ctx *fasthttp.RequestCtx) error {
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

func (sock *Socket) parseConfig() (
	newEncoderFunc NewEncoderFunc,
	newDecoderFunc NewDecoderFunc,
	dataModel dataModel,
	family family,
	address string, err error,
) {
	// example: "fasthttp:gob:unix:/run/myserver.sock"
	words := strings.SplitN(sock.Config.Address, ":", 4)

	if len(words) < 4 {
		err = ErrNotEnoughWords
		return
	}

	switch words[0] {
	case "go/net/http":
		dataModel = dataModelNetHttp
	default:
		err = errors.Wrap(ErrUnknownDataModel, words[0])
		return
	}

	var serializerType serializerType
	switch words[1] {
	case "gob":
		serializerType = serializerTypeGob
	case "json":
		serializerType = serializerTypeJSON
	default:
		err = errors.Wrap(ErrUnknownSerializer, words[1])
		return
	}

	switch words[2] {
	case "unix":
		family = familyUnix
	case "udp":
		family = familyUDP
	case "tcp":
		family = familyTCP
	default:
		err = errors.Wrap(ErrUnknownFamily, words[2])
		return
	}

	address = words[3]

	// Initializing

	if sock.Config.Logger == nil {
		sock.Config.Logger = dummyLogger
	}

	switch serializerType {
	case serializerTypeGob:
		newEncoderFunc = func(w io.Writer) Encoder {
			return gob.NewEncoder(w)
		}
		newDecoderFunc = func(r io.Reader) Decoder {
			return gob.NewDecoder(r)
		}
	case serializerTypeJSON:
		newEncoderFunc = func(w io.Writer) Encoder {
			return json.NewEncoder(w)
		}
		newDecoderFunc = func(r io.Reader) Decoder {
			return json.NewDecoder(r)
		}
	}

	return
}

func (sock *Socket) StartServer() error {
	newEncoderFunc, newDecoderFunc, dataModel, family, address, err := sock.parseConfig()
	if err != nil {
		return err
	}

	if family == familyUnix {
		os.Remove(address)
	}
	accepter, err := net.Listen(family.String(), address)
	if err != nil {
		return fmt.Errorf(`[fasthttp-socket] Cannot bind "%v:%v"\n`, family, address)
	}
	if family == familyUnix {
		if err := os.Chmod(address, sock.Config.UnixSocketPermissions); err != nil {
			return fmt.Errorf(`[fasthttp-socket] Cannot change permission on socket "%v" to "%v"`, address, sock.Config.UnixSocketPermissions)
		}
	}

	// Starting
	logger := sock.Config.Logger

	go func() {
		for {
			conn, err := accepter.Accept()
			if err != nil {
				logger.Errorf("[fasthttp-socket] got error: %v\n", err)
				continue
			}
			msg := NewMessanger(conn)

			encoder := newEncoderFunc(msg)
			decoder := newDecoderFunc(msg)
			go func() {
				logger.Print(`[fasthttp-socket-handler] opened connection`)
				sock.handleSocketConnection(encoder, decoder, dataModel)
				logger.Print(`[fasthttp-socket-handler] closed connection`)
				_ = conn.Close()
			}()
		}
	}()
	logger.Print("[fasthttp-socket] Started to listen ", address, " (", sock.Config.UnixSocketPermissions, ")")

	return nil
}

func (sock *Socket) handleSocketConnection(encoder Encoder, decoder Decoder, dataModel dataModel) {
	logger := sock.Config.Logger

	modelCodec := dataModel.GetServerCodec()

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
}

func (sock *Socket) StopServer() error {
	return ErrNotImplemented
}
