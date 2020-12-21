package kv

import (
	"bytes"
	"context"
	"os"
	"sync"
	"time"

	"github.com/boltdb/bolt"

	"github.com/projecteru2/core/types"
)

// Lithium .
type Lithium struct {
	sync.Mutex

	// Name of the root bucket.
	RootBucketKey []byte

	bolt    *bolt.DB
	path    string
	mode    os.FileMode
	timeout time.Duration
}

// NewLithium initializes a new Lithium instance.
func NewLithium() *Lithium {
	return &Lithium{
		RootBucketKey: []byte("root"),
	}
}

// Reopen re-open a kvdb file.
func (l *Lithium) Reopen(ctx context.Context) error {
	l.Lock()
	defer l.Unlock()

	if err := l.close(ctx); err != nil {
		return err
	}

	return l.open(ctx)
}

// Open opens a kvdb file.
func (l *Lithium) Open(ctx context.Context, path string, mode os.FileMode, timeout time.Duration) (err error) {
	l.Lock()
	defer l.Unlock()

	l.path = path
	l.mode = mode
	l.timeout = timeout

	return l.open(ctx)
}

func (l *Lithium) open(context.Context) (err error) {
	if l.bolt, err = bolt.Open(l.path, l.mode, &bolt.Options{Timeout: l.timeout}); err != nil {
		return
	}

	err = l.bolt.Update(func(tx *bolt.Tx) error {
		_, ce := tx.CreateBucketIfNotExists(l.RootBucketKey)
		return ce
	})

	return
}

// Close closes the kvdb file.
func (l *Lithium) Close(ctx context.Context) error {
	l.Lock()
	defer l.Unlock()
	return l.close(ctx)
}

func (l *Lithium) close(context.Context) error {
	return l.bolt.Close()
}

// Put creates/updates a key/value pair.
func (l *Lithium) Put(ctx context.Context, key []byte, value []byte) (err error) {
	l.Lock()
	defer l.Unlock()

	return l.update(func(bkt *bolt.Bucket) error {
		return bkt.Put(key, value)
	})
}

// Get read a key's value.
func (l *Lithium) Get(ctx context.Context, key []byte) (dst []byte, err error) {
	l.Lock()
	defer l.Unlock()

	err = l.view(func(bkt *bolt.Bucket) error {
		src := bkt.Get(key)
		dst = make([]byte, len(src))

		for n := 0; n < len(dst); {
			n += copy(dst, src)
		}

		return nil
	})

	return
}

// Delete deletes a key.
func (l *Lithium) Delete(ctx context.Context, key []byte) error {
	l.Lock()
	defer l.Unlock()
	return l.update(func(bkt *bolt.Bucket) error {
		return bkt.Delete(key)
	})
}

// Scan scans all the key/value pairs.
func (l *Lithium) Scan(ctx context.Context, prefix []byte) (<-chan ScanEntry, func()) {
	ch := make(chan ScanEntry)
	locked := make(chan struct{})

	exit := make(chan struct{})
	abort := func() {
		close(exit)
	}

	go func() {
		defer close(ch)

		l.Lock()
		defer l.Unlock()

		close(locked)
		ent := &LithiumScanEntry{}

		scan := func(bkt *bolt.Bucket) error {
			c := bkt.Cursor()
			for key, value := c.First(); key != nil && bytes.HasPrefix(key, prefix); key, value = c.Next() {
				ent.key = key
				ent.value = value

				select {
				case <-exit:
					return nil
				case ch <- *ent:
				}
			}
			return nil
		}

		if err := l.view(scan); err != nil {
			ent.err = err
			select {
			case <-exit:
			case ch <- *ent:
			}
		}
	}()

	// Makes sure that the scan goroutine has been locked.
	<-locked

	return ch, abort
}

// NextSequence generates a new sequence.
func (l *Lithium) NextSequence(context.Context) (uint64, error) {
	l.Lock()
	defer l.Unlock()

	var seq uint64
	err := l.update(func(bkt *bolt.Bucket) (ue error) {
		seq, ue = bkt.NextSequence()
		return
	})

	return seq, err
}

func (l *Lithium) view(fn func(*bolt.Bucket) error) error {
	return l.bolt.Update(func(tx *bolt.Tx) error {
		bkt, err := l.getBucket(tx, l.RootBucketKey)
		if err != nil {
			return err
		}
		return fn(bkt)
	})
}

func (l *Lithium) update(fn func(*bolt.Bucket) error) error {
	return l.bolt.Update(func(tx *bolt.Tx) error {
		bkt, err := l.getBucket(tx, l.RootBucketKey)
		if err != nil {
			return err
		}
		return fn(bkt)
	})
}

func (l *Lithium) getBucket(tx *bolt.Tx, key []byte) (bkt *bolt.Bucket, err error) {
	bkt = tx.Bucket(l.RootBucketKey)
	if bkt == nil {
		err = types.NewDetailedErr(types.ErrInvalidWALBucket, key)
	}
	return
}

// LithiumScanEntry indicates an entry of scanning.
type LithiumScanEntry struct {
	err   error
	key   []byte
	value []byte
}

// Pair means a pair of key/value.
func (e LithiumScanEntry) Pair() ([]byte, []byte) {
	return e.key, e.value
}

// Error .
func (e LithiumScanEntry) Error() error {
	return e.err
}
