package fasthttpsocket

type Logger interface {
	Print(args ...interface{})
	Errorf(format string, args ...interface{})
}

type DummyLogger struct{}

func (l *DummyLogger) Print(args ...interface{})                 {}
func (l *DummyLogger) Errorf(format string, args ...interface{}) {}

var (
	dummyLogger = &DummyLogger{}
)
