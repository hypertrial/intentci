package scheduler

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

func TestPrefixAndIndex(t *testing.T) {
	if indexByte([]byte("ab"), 'x') != -1 {
		t.Fatal()
	}
	if indexByte([]byte("ab"), 'a') != 0 {
		t.Fatal()
	}
	var buf bytes.Buffer
	w := prefixWriter(&buf, "id")
	_, _ = w.Write([]byte("no-newline"))
	flushPrefix(w)
	flushPrefix(nil)
	flushPrefix(&buf)
	var mu sync.Mutex
	lw := lockedWriter{w: nil, mu: &mu}
	if lw.writer() != nil {
		t.Fatal()
	}
	errW := lockedWriter{w: errWriter{}, mu: &mu}
	pw := prefixWriter(errW, "x")
	_, _ = pw.Write([]byte("line\n"))
}

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("fail") }
