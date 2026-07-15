package pdf

import (
	"fmt"
	"io"
)

type XrefEntry struct {
	offset     int64
	generation int
	inUse      bool
}

func (e XrefEntry) Serialize(w io.Writer) error {
	s := 'f'
	if e.inUse {
		s = 'n'
	}
	_, err := fmt.Fprintf(w, "%010d %05d %c\r\n", e.offset, e.generation, s)
	return err
}

type XrefTable struct {
	entries []XrefEntry
}

func (t XrefTable) Serialize(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "xref\n0 %d\n", len(t.entries)); err != nil {
		return err
	}
	for _, e := range t.entries {
		if err := e.Serialize(w); err != nil {
			return err
		}
	}
	return nil
}

func newXrefTable() *XrefTable {
	entries := []XrefEntry{
		{
			offset:     0,
			generation: 2<<16 - 1,
			inUse:      false,
		},
	}
	return &XrefTable{entries: entries}
}

func (pw *Writer) writeTrailer(infoRef Ref) error {
	xrefOffset := pw.w.Count()
	size := Integer(len(pw.xref.entries))
	root := pw.catalog

	trailer := Dict{keys: make([]Name, 0), values: make(map[Name]Object)}

	trailer.Set(Name("Size"), size)
	trailer.Set(Name("Root"), root)
	if infoRef.id > 0 {
		trailer.Set(Name("Info"), infoRef)
	}

	if _, err := fmt.Fprintf(pw.w, "trailer\n"); err != nil {
		return err
	}
	if err := trailer.Serialize(pw.w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(pw.w, "startxref\n%d\n", xrefOffset); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(pw.w, "%%EOF\n"); err != nil {
		return err
	}

	return nil
}
