package fasthttpsocket

import (
	"bufio"
	"bytes"
	"sync"

	"github.com/trafficstars/fasthttp"
)

var _ ClientCodec = newClientCodecRaw()
var _ ServerCodec = newServerCodecRaw()

type modelRawRequest struct {
	codec *modelCodecRaw
	Data  []byte
}

func (model *modelRawRequest) Release() {
	model.Reset()
	model.codec.requestPool.Put(model)
}

func (model *modelRawRequest) IsRequest() bool {
	return true
}

func (model *modelRawRequest) Reset() {
	model.Data = nil
}

func (model *modelRawRequest) Marshal() []byte {
	return model.Data
}

func (model *modelRawRequest) Unmarshal(b []byte) {
	model.Data = b
}

type modelRawResponse struct {
	codec *modelCodecRaw
	Data  []byte
}

func (model *modelRawResponse) Release() {
	model.Reset()
	model.codec.responsePool.Put(model)
}

func (model *modelRawResponse) IsResponse() bool {
	return true
}

func (model *modelRawResponse) Reset() {
	model.Data = nil
}

func (model *modelRawResponse) Marshal() []byte {
	return model.Data
}

func (model *modelRawResponse) Unmarshal(b []byte) {
	model.Data = b
}

type modelCodecRaw struct {
	requestPool  *sync.Pool
	responsePool *sync.Pool
	buf          bytes.Buffer
}

func newModelCodecRaw() *modelCodecRaw {
	codec := &modelCodecRaw{}
	codec.requestPool = &sync.Pool{
		New: func() interface{} {
			r := &modelRawRequest{
				codec: codec,
			}

			return r
		},
	}
	codec.responsePool = &sync.Pool{
		New: func() interface{} {
			return &modelRawResponse{
				codec: codec,
			}
		},
	}
	return codec
}

func (codec *modelCodecRaw) GetRequest() TransmittableRequest {
	return codec.requestPool.Get().(TransmittableRequest)
}

func (codec *modelCodecRaw) GetResponse() TransmittableResponse {
	return codec.responsePool.Get().(TransmittableResponse)
}

type ClientCodecRaw struct {
	*modelCodecRaw
}

func newClientCodecRaw() *ClientCodecRaw {
	codec := &ClientCodecRaw{}
	codec.modelCodecRaw = newModelCodecRaw()
	return codec
}

func (codec *ClientCodecRaw) Encode(modelI TransmittableRequest, ctx *fasthttp.RequestCtx) error {
	src := &ctx.Request
	dst := modelI.(*modelRawRequest)

	codec.buf.Reset()
	_, err := src.WriteTo(&codec.buf)
	if err != nil {
		return err
	}

	dst.Data = codec.buf.Bytes()
	return nil
}

func (codec *ClientCodecRaw) Decode(ctx *fasthttp.RequestCtx, modelI TransmittableResponse) (err error) {
	src := modelI.(*modelRawResponse)
	dst := &ctx.Response

	dst.Reset()
	err = dst.Read(bufio.NewReaderSize(bytes.NewReader(src.Data), len(src.Data)))
	if err != nil {
		return err
	}

	return
}

type ServerCodecRaw struct {
	*modelCodecRaw
}

func newServerCodecRaw() *ServerCodecRaw {
	codec := &ServerCodecRaw{}
	codec.modelCodecRaw = newModelCodecRaw()
	return codec
}

func (codec *ServerCodecRaw) Decode(ctx *fasthttp.RequestCtx, modelI TransmittableRequest) (err error) {
	src := modelI.(*modelRawRequest)
	dst := &ctx.Request

	dst.Reset()
	err = dst.Read(bufio.NewReaderSize(bytes.NewReader(src.Data), len(src.Data)))
	if err != nil {
		return err
	}

	return
}

func (codec *ServerCodecRaw) Encode(modelI TransmittableResponse, ctx *fasthttp.RequestCtx) error {
	src := &ctx.Response
	dst := modelI.(*modelRawResponse)

	codec.buf.Reset()
	_, err := src.WriteTo(&codec.buf)
	if err != nil {
		return err
	}

	dst.Data = codec.buf.Bytes()
	return nil
}
