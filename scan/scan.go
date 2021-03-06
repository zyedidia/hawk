package scan

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
)

// endOfSource is returned when io.EOF is reached in one of
// the sources and there are still more sources to process.
var endOfSource = errors.New("end of source")

// Source is the interface thats wraps io.Reader and provides
// the Name method.
type Source interface {
	io.Reader

	// Name returns the name of the source.
	Name() string
}

// MultiSource returns a Source that's the logical concatenation
// of the provided input sources.
func MultiSource(sources ...Source) Source {
	return &multiSource{sources}
}

type multiSource struct {
	sources []Source
}

func (ms *multiSource) Read(p []byte) (n int, err error) {
	for len(ms.sources) > 0 {
		n, err = ms.sources[0].Read(p)
		if err == io.EOF {
			ms.sources = ms.sources[1:]
			if len(ms.sources) > 0 {
				err = endOfSource
				return
			}
		}
		if n > 0 || err != nil {
			return
		}
	}
	return 0, io.EOF
}

func (ms *multiSource) Name() string {
	if len(ms.sources) == 0 {
		return ""
	}
	return ms.sources[0].Name()
}

// A Scanner is used for splitting input into rows and
// splitting rows into fields.
type Scanner struct {
	lr       lineReader
	rowsRx   *regexp.Regexp
	fieldsRx *regexp.Regexp
	err      error // sticky err

	recNumber     int
	fileRecNumber int
	rec           string
	fields        []string
}

// SetSource sets a Source for scanner to read from.
func (sc *Scanner) SetSource(src Source) {
	if sc.rowsRx != nil {
		sc.lr = newRxLineReader(src, sc.rowsRx)
	} else {
		sc.lr = newSimpleLineReader(src)
	}
	sc.recNumber = 0
}

// SetRowSep sets regexp rx that will be used to separate
// input into rows.
func (sc *Scanner) SetRowSep(rx string) {
	if sc.err != nil || rx == "" {
		return
	}
	rs, err := regexp.Compile(rx)
	if err != nil {
		sc.err = fmt.Errorf("setting RS: %v", err)
		return
	}
	sc.rowsRx = rs
	if sc.lr != nil {
		sc.lr = newRxLineReader(sc.lr, sc.rowsRx)
	}
}

// SetFieldSep sets regexp rx that will be used to separate
// row into fields.
func (sc *Scanner) SetFieldSep(rx string) {
	if sc.err != nil {
		return
	}
	fs, err := regexp.Compile(rx)
	if err != nil {
		sc.err = fmt.Errorf("setting FS: %v", err)
		return
	}
	sc.fieldsRx = fs
}

// Scan scans another record and parses it into fields. It there
// is an error or EOF is reached, Scan returns false. Otherwise
// it returns true.
func (sc *Scanner) Scan() bool {
	if sc.err != nil {
		return false
	}
	if sc.lr == nil {
		sc.err = errors.New("scan: nil reader")
		return false
	}

	data, err := ioutil.ReadAll(sc.lr)
	// line, err := sc.lr.ReadLine()
	// if err == io.EOF {
	// 	return false
	// } else if err == endOfSource {
	// 	sc.fileRecNumber = 0
	// 	goto readRecord
	// } else if err != nil {
	// 	sc.err = err
	// 	return false
	// }
	fail := len(data) == 0 || (err != nil && err != io.EOF)

	if !fail {
		sc.splitRecord(data)
		sc.recNumber++
		sc.fileRecNumber++
	}

	return !fail
}

func (sc *Scanner) splitRecord(rec []byte) {
	sc.rec = string(rec)
	if sc.fieldsRx != nil {
		sc.fields = sc.fieldsRx.Split(sc.rec, -1)
		if len(sc.fields) > 0 && sc.fields[0] == "" {
			sc.fields = sc.fields[1:]
		}
		if len(sc.fields) > 0 && sc.fields[len(sc.fields)-1] == "" {
			sc.fields = sc.fields[:len(sc.fields)-1]
		}
	} else {
		sc.fields = strings.Fields(sc.rec)
	}
}

func (sc *Scanner) Err() error {
	return sc.err
}

// Field returns ith field from the current row. If i > NF, Field
// returns an empty string. If i == 0, Field returns the whole record.
// Field panics if i < 0.
func (sc *Scanner) Field(i int) string {
	if sc.err != nil {
		return ""
	}
	switch {
	case i < 0:
		panic("negative field index")
	case i == 0:
		return sc.rec
	case i <= len(sc.fields):
		return sc.fields[i-1]
	}
	return ""
}

// RecordNumber returns the current record number.
func (sc *Scanner) RecordNumber() int {
	return sc.recNumber
}

// FieldCount returns number of fields of the current row.
func (sc *Scanner) FieldCount() int {
	return len(sc.fields)
}

// Filename returns the name of the currently processed source.
func (sc *Scanner) Filename() string {
	if sc.lr == nil {
		return ""
	}
	return sc.lr.Name()
}

// FileRecordNumber returns the current record number in the currently
// processed file.
func (sc *Scanner) FileRecordNumber() int {
	return sc.fileRecNumber
}

// lineReader returns a non-empty slice if and only if
// the error is nil.
type lineReader interface {
	Source // to be able to read buffered data
	ReadLine() ([]byte, error)
}

type simpleLineReader struct {
	src  Source
	name string // name of the current source
	br   *bufio.Reader
}

func newSimpleLineReader(src Source) *simpleLineReader {
	return &simpleLineReader{
		src:  src,
		name: src.Name(),
		br:   bufio.NewReader(src),
	}
}

func (sr *simpleLineReader) Read(p []byte) (n int, err error) {
	n, err = sr.br.Read(p)
	if err == endOfSource {
		sr.name = sr.src.Name()
	}
	return
}

func (sr *simpleLineReader) Name() string { return sr.name }

func (sr *simpleLineReader) ReadLine() ([]byte, error) {
	line, err := sr.br.ReadBytes('\n')
	if len(line) > 0 {
		line = line[:len(line)-1] // remove '\n'
	}
	return line, err
}

const bufSize = 4096

var _bufSize = bufSize // for testing purposes

type rxLineReader struct {
	buf  [bufSize]byte
	ptr  []byte
	src  Source
	name string // name of the current source
	rx   *regexp.Regexp
	stat int
}

// stats
const (
	sourceEnd = 1
	finished  = 2
)

func newRxLineReader(src Source, sepRx *regexp.Regexp) *rxLineReader {
	return &rxLineReader{
		src:  src,
		name: src.Name(),
		rx:   sepRx,
	}
}

func (rr *rxLineReader) Read(p []byte) (n int, err error) {
	if len(rr.ptr) > 0 {
		n := copy(p, rr.ptr)
		rr.ptr = rr.ptr[n:]
		return n, nil
	}
	return rr.src.Read(p)
}

func (rr *rxLineReader) Name() string {
	if len(rr.ptr) > 0 {
		return rr.name
	}
	return rr.src.Name()
}

func (rr *rxLineReader) ReadLine() (line []byte, err error) {
	var loc []int
	for {
		if len(rr.ptr) == 0 {
			if err := rr.loadBuf(); err != nil {
				return nil, err
			}
			if rr.stat >= sourceEnd && len(rr.ptr) == 0 {
				if len(line) > 0 && loc != nil {
					line = line[:loc[0]]
				}
				if rr.stat >= finished {
					if len(line) > 0 {
						return line, nil
					}
					return line, io.EOF
				}
				rr.stat = 0
				return line, nil
			}
		}
		line = append(line, rr.ptr...)
		loc = rr.rx.FindIndex(line)

		if loc == nil || loc[1] == len(line) {
			rr.ptr = nil
			continue
		}
		rr.ptr = line[loc[1]:]
		return line[:loc[0]], nil
	}
}

func (rr *rxLineReader) loadBuf() error { return rr.loadBufN(_bufSize) }

func (rr *rxLineReader) loadBufN(n int) error {
	if rr.stat >= sourceEnd {
		return nil
	}
	m, err := rr.src.Read(rr.buf[:n])
	rr.ptr = rr.buf[:m]
	switch {
	case err == io.EOF:
		rr.stat = finished
	case err == endOfSource:
		rr.name = rr.src.Name()
		rr.stat = sourceEnd
	case err != nil:
		return err
	case m == 0:
		return errors.New("scan: empty read")
	}
	return nil
}
