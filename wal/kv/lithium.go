package kv

import (
	"bytes"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"go.etcd.io/bbolt"

	"github.com/projecteru2/core/types"
)

// Lithium is a bbolt backed KV.
type Lithium struct {
	sync.Mutex

	RootBucketKey []byte

	bolt    *bbolt.DB
	path    string
	mode    os.FileMode
	timeout time.Duration
}

func NewLithium() *Lithium {
	return &Lithium{
		RootBucketKey: []byte("root"),
	}
}

// Reopen closes and reopens the kvdb file.
func (l *Lithium) Reopen() error {
	l.Lock()
	defer l.Unlock()

	if err := l.close(); err != nil {
		return err
	}

	return l.open()
}

func (l *Lithium) Open(path string, mode os.FileMode, timeout time.Duration) (err error) {
	l.Lock()
	defer l.Unlock()

	l.path = path
	l.mode = mode
	l.timeout = timeout

	return l.open()
}

func (l *Lithium) Close() error {
	l.Lock()
	defer l.Unlock()
	return l.close()
}

func (l *Lithium) Put(key, value []byte) (err error) {
	return l.update(func(bkt *bbolt.Bucket) error {
		return bkt.Put(key, value)
	})
}

func (l *Lithium) Get(key []byte) (dst []byte, err error) {
	err = l.update(func(bkt *bbolt.Bucket) error {
		src := bkt.Get(key)
		dst = make([]byte, len(src))
		copy(dst, src)
		return nil
	})

	return dst, err
}

func (l *Lithium) Delete(key []byte) error {
	return l.update(func(bkt *bbolt.Bucket) error {
		return bkt.Delete(key)
	})
}

func (l *Lithium) Scan(prefix []byte) (<-chan ScanEntry, func()) {
	ch := make(chan ScanEntry)

	exit := make(chan struct{})
	abort := func() {
		close(exit)
	}

	go func() {
		defer close(ch)

		scan := func(bkt *bbolt.Bucket) error {
			c := bkt.Cursor()
			for key, value := c.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, value = c.Next() {
				select {
				case <-exit:
					return nil
				case ch <- LithiumScanEntry{key: key, value: value}:
				}
			}
			return nil
		}

		if err := l.update(scan); err != nil {
			select {
			case <-exit:
			case ch <- LithiumScanEntry{err: err}:
			}
		}
	}()

	return ch, abort
}

func (l *Lithium) NextSequence() (uint64, error) {
	var seq uint64
	err := l.update(func(bkt *bbolt.Bucket) (ue error) {
		seq, ue = bkt.NextSequence()
		return ue
	})

	return seq, err
}

func (l *Lithium) open() (err error) {
	if l.bolt, err = bbolt.Open(l.path, l.mode, &bbolt.Options{Timeout: l.timeout}); err != nil {
		return err
	}

	err = l.bolt.Update(func(tx *bbolt.Tx) error {
		_, ce := tx.CreateBucketIfNotExists(l.RootBucketKey)
		return ce
	})

	return err
}

func (l *Lithium) close() error {
	return l.bolt.Close()
}

func (l *Lithium) update(fn func(*bbolt.Bucket) error) error {
	return l.bolt.Update(func(tx *bbolt.Tx) error {
		bkt, err := l.getBucket(tx, l.RootBucketKey)
		if err != nil {
			return err
		}
		return fn(bkt)
	})
}

func (l *Lithium) getBucket(tx *bbolt.Tx, key []byte) (bkt *bbolt.Bucket, err error) {
	bkt = tx.Bucket(l.RootBucketKey)
	if bkt == nil {
		err = errors.Wrapf(types.ErrInvalidWALBucket, "%+v", key)
	}
	return bkt, err
}

// LithiumScanEntry is a key/value pair produced by Lithium.Scan.
type LithiumScanEntry struct {
	err   error
	key   []byte
	value []byte
}

func (e LithiumScanEntry) Pair() ([]byte, []byte) {
	return e.key, e.value
}

func (e LithiumScanEntry) Error() error {
	return e.err
}
