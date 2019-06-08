package fasthttpsocket

import (
	"os"
)

type Config struct {
	Address               string
	UnixSocketPermissions os.FileMode
	Logger                Logger
}
