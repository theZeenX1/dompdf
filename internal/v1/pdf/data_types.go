package pdf

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// main interface to serialize all objects to pdf data types
type Object interface {
	Serialize(w io.Writer) error
}

// basic data types:
type Integer int

func (i Integer) Serialize(w io.Writer) error {
	_, e := fmt.Fprintf(w, "%d", int(i))
	return e
}

type Real float64

func (f Real) Serialize(w io.Writer) error {
	_, e := fmt.Fprintf(w, "%f", float64(f))
	return e
}

type Boolean bool

func (b Boolean) Serialize(w io.Writer) error {
	s := "false"
	if bool(b) {
		s = "true"
	}
	_, e := fmt.Fprint(w, s)
	return e
}

type Null struct{}

func (n Null) Serialize(w io.Writer) error {
	_, e := fmt.Fprintf(w, "null")
	return e
}

// name is a pdf variable ("/Name")
type Name string

func (n Name) Serialize(w io.Writer) error {
	_, e := fmt.Fprintf(w, "/%s", n)
	return e
}

// serializes to (...) notation
type String string

func (s String) Serialize(w io.Writer) error {
	b := strings.Builder{}
	for _, c := range string(s) {
		switch c {
		case '(', ')', '\\', '\r', '\n':
			b.WriteByte('\\')
			b.WriteByte(byte(c))
		default:
			b.WriteByte(byte(c))
		}
	}
	_, e := fmt.Fprintf(w, "(%s)", b.String())
	return e
}

// serializes to <4E567..> notation
type HexString string

func (s HexString) Serialize(w io.Writer) (int64, error) {
	encoded := hex.EncodeToString([]byte(s))
	n, err := fmt.Fprintf(w, "<%s>", strings.ToUpper(encoded))
	return int64(n), err
}

// composite data types

type Array []Object

func (a Array) Serialize(w io.Writer) error {
	fmt.Fprint(w, "[")
	for _, o := range a {
		fmt.Fprint(w, " ")
		if err := o.Serialize(w); err != nil {
			return err
		}
	}
	_, e := fmt.Fprint(w, "]")
	return e
}

// serializes to << key value >>
type Dict struct {
	keys   []Name
	values map[Name]Object
}

func NewDict() *Dict {
	return &Dict{
		values: make(map[Name]Object),
	}
}

// sets k: v, overrides previous value
func (d *Dict) Set(k Name, v Object) {
	if _, exists := d.values[k]; !exists {
		d.keys = append(d.keys, k)
	}
	d.values[k] = v
}

func (d *Dict) Serialize(w io.Writer) error {
	fmt.Fprint(w, "<<\n")
	for _, k := range d.keys {
		if err := k.Serialize(w); err != nil {
			return err
		}
		if err := d.values[k].Serialize(w); err != nil {
			return err
		}
		fmt.Fprint(w, "\n")
	}
	_, e := fmt.Fprint(w, ">>")
	return e
}

type Ref struct {
	id  int
	gen int
}

func (r Ref) Serialize(w io.Writer) error {
	_, e := fmt.Fprintf(w, "%d %d R", r.id, r.gen)
	return e
}

// actual PDFObject
type PDFObject struct {
	id  int
	gen int
	Val Object
}

func (ind *PDFObject) Serialize(w io.Writer) error {
	fmt.Fprintf(w, "%d %d obj\n", ind.id, ind.gen)
	if err := ind.Val.Serialize(w); err != nil {
		return err
	}
	_, e := fmt.Fprint(w, "\nendobj\n")
	return e
}

func (ind *PDFObject) Ref() Ref {
	return Ref{
		id:  ind.id,
		gen: ind.gen,
	}
}

// pdf streams
type StreamObj struct {
	dict    Dict
	content []byte
}

func (s *StreamObj) Serialize(w io.Writer) error {
	d := s.dict
	d.Set(Name("Length"), Integer(len(s.content)))
	if err := d.Serialize(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nstream\n"); err != nil {
		return err
	}
	if _, err := w.Write(s.content); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nendstream"); err != nil {
		return err
	}

	return nil
}
