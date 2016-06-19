package calcium

import "io"
import "io/ioutil"

func ensureReaderClosed(stream io.ReadCloser) {
	if stream == nil {
		return
	}
	io.CopyN(ioutil.Discard, stream, 512)
	stream.Close()
}
