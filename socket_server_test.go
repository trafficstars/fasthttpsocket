package fasthttpsocket

import (
	"fmt"
	"testing"
	"time"

	"github.com/trafficstars/fasthttp"
	"github.com/stretchr/testify/assert"
)

const (
	testUnixAddress = "raw:native:unixpacket:/tmp/.fasthttpsocket_test"
)

type testErrorLogger struct{
	t *testing.T
}

func (l *testErrorLogger) Errorf(fm string, args ...interface{}) {
	panic(fmt.Errorf(fm, args...))
}

func (l *testErrorLogger) Print(args ...interface{}) {
	//l.t.Errorf("%v", args)
}

type testHandleRequester struct {}
func (h *testHandleRequester) HandleRequest(ctx *fasthttp.RequestCtx) error {
	return nil
}

func TestUnixGram(t *testing.T) {
	srv, err := NewSocketServer(&testHandleRequester{}, Config{
		Address:               testUnixAddress,
		UnixSocketPermissions: 0700,
		Logger: &testErrorLogger{t},
	})
	assert.NoError(t, err)

	err = srv.Start()
	assert.NoError(t, err)

	client, err := NewSocketClient(Config{
		Address: testUnixAddress,
		Logger: &testErrorLogger{t},
	})
	assert.NoError(t, err)

	err = client.Start(1)
	assert.NoError(t, err)

	go func( ){
		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.Header.Set(`Host`, `trafficstars.com`)
		err = client.SendAndReceive(reqCtx)
		assert.NoError(t, err)
	}()
	time.Sleep(time.Second)
}
