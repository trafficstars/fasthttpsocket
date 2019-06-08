package fasthttpsocket

import (
	"encoding/gob"
	"encoding/json"
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/trafficstars/fasthttp"
)

var (
	ErrUnknownFamily     = errors.New(`[fasthttp-socket] unknown family/transport`)
	ErrUnknownSerializer = errors.New(`[fasthttp-socket] unknown serializer`)
	ErrUnknownDataModel  = errors.New(`[fasthttp-socket] unknown data model`)
	ErrNotEnoughWords    = errors.New(`[fasthttp-socket] invalid address, expected syntax "datamodel:serializer:family:address", example "go/net/http:gob:unix:/run/myserver.sock"`)
	ErrNotImplemented    = errors.New(`[fasthttp-socket] not implemented, yet`)
)

type Family int

const (
	FamilyUnix = iota
	FamilyUDP
	FamilyTCP
)

func (f Family) String() string {
	switch f {
	case FamilyUnix:
		return `unix`
	case FamilyUDP:
		return `udp`
	case FamilyTCP:
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

func parseConfig(cfg *Config) (
	newEncoderFunc NewEncoderFunc,
	newDecoderFunc NewDecoderFunc,
	dataModel dataModel,
	family Family,
	address string, err error,
) {
	// example: "fasthttp:gob:unix:/run/myserver.sock"
	words := strings.SplitN(cfg.Address, ":", 4)

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
		family = FamilyUnix
	case "udp":
		family = FamilyUDP
	case "tcp":
		family = FamilyTCP
	default:
		err = errors.Wrap(ErrUnknownFamily, words[2])
		return
	}

	address = words[3]

	// Initializing

	if cfg.Logger == nil {
		cfg.Logger = dummyLogger
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
