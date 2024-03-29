package fasthttpsocket

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/trafficstars/fasthttp"
)

func init() {
	gob.Register(ioutil.NopCloser(nil))
	var i io.Reader
	i = &BytesReader{}
	gob.Register(i)
	gob.Register(http.NoBody)
}

var _ ClientCodec = newClientCodecNetHttp()
var _ ServerCodec = newServerCodecNetHttp()

type modelNetHttpRequest struct {
	codec *modelCodecNetHttp
	http.Request
}

func (model *modelNetHttpRequest) Release() {
	model.Reset()
	model.codec.requestPool.Put(model)
}

func (model *modelNetHttpRequest) IsRequest() bool {
	return true
}

func (model *modelNetHttpRequest) Reset() {
	model.Request = http.Request{}
}

type modelNetHttpResponse struct {
	codec *modelCodecNetHttp
	http.Response
}

func (model *modelNetHttpResponse) Release() {
	model.Reset()
	model.codec.responsePool.Put(model)
}

func (model *modelNetHttpResponse) IsResponse() bool {
	return true
}

func (model *modelNetHttpResponse) Reset() {
	model.Response = http.Response{}
}

type modelCodecNetHttp struct {
	requestPool  *sync.Pool
	responsePool *sync.Pool
	buf          bytes.Buffer
}

func newModelCodecNetHttp() *modelCodecNetHttp {
	codec := &modelCodecNetHttp{}
	codec.requestPool = &sync.Pool{
		New: func() interface{} {
			r := &modelNetHttpRequest{
				codec: codec,
			}

			req := r.Request
			req.Proto = "1.1"
			req.ProtoMajor = 1
			req.ProtoMinor = 1

			return r
		},
	}
	codec.responsePool = &sync.Pool{
		New: func() interface{} {
			return &modelNetHttpResponse{
				codec: codec,
			}
		},
	}
	return codec
}

func (codec *modelCodecNetHttp) GetRequest() TransmittableRequest {
	return codec.requestPool.Get().(TransmittableRequest)
}

func (codec *modelCodecNetHttp) GetResponse() TransmittableResponse {
	return codec.responsePool.Get().(TransmittableResponse)
}

type ClientCodecNetHttp struct {
	*modelCodecNetHttp
}

func newClientCodecNetHttp() *ClientCodecNetHttp {
	codec := &ClientCodecNetHttp{}
	codec.modelCodecNetHttp = newModelCodecNetHttp()
	return codec
}

type BytesReader struct {
	Bytes []byte
	pos   int
}

func NewBytesReader(b []byte) *BytesReader {
	return &BytesReader{Bytes: b}
}

func (r *BytesReader) Read(b []byte) (int, error) {
	var err error
	i := 0
	for ; i < len(b) && r.pos+i < len(r.Bytes); i++ {
		b[i] = r.Bytes[r.pos+i]
	}
	if i < len(b) {
		err = io.EOF
	}
	r.pos += i
	return i, err
}

func (r *BytesReader) Close() error {
	return nil
}

func (codec *ClientCodecNetHttp) Encode(modelI TransmittableRequest, ctx *fasthttp.RequestCtx) error {
	src := &ctx.Request
	dst := &modelI.(*modelNetHttpRequest).Request
	/*dst.Method = string(ctx.Method())
	dst.URL, _ = url.Parse(string(src.URI().FullURI()))
	hdr := src.Header
	hdr.*/

	// Just a simple way (slow, but...)

	codec.buf.Reset()
	_, err := src.WriteTo(&codec.buf)
	if err != nil {
		return err
	}
	parsedRequest, err := http.ReadRequest(bufio.NewReader(&codec.buf))
	*dst = *parsedRequest

	dst.Body = NewBytesReader(src.Body())

	return nil
}

func (codec *ClientCodecNetHttp) Decode(ctx *fasthttp.RequestCtx, modelI TransmittableResponse) (err error) {
	src := &modelI.(*modelNetHttpResponse).Response
	dst := &ctx.Response

	// Just a simple way (slow, but...)

	codec.buf.Reset()
	err = src.Write(&codec.buf)
	if err != nil {
		return err
	}

	dst.Reset()
	err = dst.Read(bufio.NewReader(&codec.buf))
	if err != nil {
		return err
	}

	return
}

type ServerCodecNetHttp struct {
	*modelCodecNetHttp
}

func newServerCodecNetHttp() *ServerCodecNetHttp {
	codec := &ServerCodecNetHttp{}
	codec.modelCodecNetHttp = newModelCodecNetHttp()
	return codec
}

func (codec *ServerCodecNetHttp) Decode(ctx *fasthttp.RequestCtx, modelI TransmittableRequest) (err error) {
	src := &modelI.(*modelNetHttpRequest).Request
	dst := &ctx.Request

	// Just a simple way (slow, but...)

	codec.buf.Reset()
	err = src.Write(&codec.buf)
	if err != nil {
		return err
	}

	dst.Reset()
	err = dst.Read(bufio.NewReader(&codec.buf))
	if err != nil {
		return err
	}

	return
}

func (codec *ServerCodecNetHttp) Encode(modelI TransmittableResponse, ctx *fasthttp.RequestCtx) error {
	src := &ctx.Response
	dst := &modelI.(*modelNetHttpResponse).Response

	// Just a simple way (slow, but...)

	codec.buf.Reset()
	_, err := src.WriteTo(&codec.buf)
	if err != nil {
		return err
	}
	parsedResponse, err := http.ReadResponse(bufio.NewReader(&codec.buf), nil)
	if err != nil {
		return err
	}
	*dst = *parsedResponse

	dst.Body = NewBytesReader(src.Body())

	return nil
}
