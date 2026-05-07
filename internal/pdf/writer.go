package pdf

import (
	"bufio"
	"fmt"
	"io"
)

type countWriter struct {
	*bufio.Writer
	cnt int
}

func (cw *countWriter) Count() int {
	return cw.cnt
}

func (cw *countWriter) Write(p []byte) (int, error) {
	n, err := cw.Writer.Write(p)
	cw.cnt += n
	return n, err
}

func (cw *countWriter) Flush() error {
	return cw.Writer.Flush()
}

func (cw *countWriter) WriteString(s string) error {
	_, err := fmt.Fprint(cw, s)
	return err
}

type Writer struct {
	w       *countWriter
	objects []*Ref
	pages   []*Ref
	// objectMap        map[int]*PDFObject // id -> pdfobj
	fonts            map[string]*Ref // name -> ref
	images           map[string]*Ref // name -> ref
	info             *DocumentInfo
	xref             *XrefTable
	catalog          *Ref
	pageRoot         *Ref
	currObjectNumber int
}

func NewWriter(w io.Writer, info *DocumentInfo) *Writer {
	cw := &countWriter{Writer: bufio.NewWriter(w)}
	pw := Writer{
		w:    cw,
		xref: newXrefTable(),
		// objectMap: make(map[int]*PDFObject),
		fonts:  make(map[string]*Ref),
		images: make(map[string]*Ref),
		info:   info,
	}
	return &pw
}
