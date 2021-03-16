package types

import (
	"bytes"
	"io"
	"io/ioutil"
	"sync"

	"github.com/pkg/errors"
)

// ReaderManager return Reader under concurrency
type ReaderManager interface {
	GetReader() (io.Reader, error)
}

// NewReaderManager converts Reader to ReadSeeker
func NewReaderManager(r io.Reader) (ReaderManager, error) {
	bs, err := ioutil.ReadAll(r)
	return &readerManager{
		r: bytes.NewReader(bs),
	}, errors.WithStack(err)
}

type readerManager struct {
	mux sync.Mutex
	r   io.ReadSeeker
}

func (rm *readerManager) GetReader() (_ io.Reader, err error) {
	rm.mux.Lock()
	defer rm.mux.Unlock()
	buf := &bytes.Buffer{}
	if _, err = io.Copy(buf, rm.r); err != nil {
		return nil, errors.WithStack(err)
	}
	_, err = rm.r.Seek(0, io.SeekStart)
	return buf, errors.WithStack(err)
}
