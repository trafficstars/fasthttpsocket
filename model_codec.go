package fasthttpsocket

import (
	"github.com/trafficstars/fasthttp"
)

type TransmittableModel interface {
	Reset()
	Release()
}

type TransmittableRequest interface {
	TransmittableModel
	IsRequest() bool
}

type TransmittableResponse interface {
	TransmittableModel
	IsResponse() bool
}

type ClientCodec interface {
	Encode(TransmittableRequest, *fasthttp.RequestCtx) error
	Decode(*fasthttp.RequestCtx, TransmittableResponse) error
	GetRequest() TransmittableRequest
	GetResponse() TransmittableResponse
}

type ServerCodec interface {
	Encode(TransmittableResponse, *fasthttp.RequestCtx) error
	Decode(*fasthttp.RequestCtx, TransmittableRequest) error
	GetRequest() TransmittableRequest
	GetResponse() TransmittableResponse
}
