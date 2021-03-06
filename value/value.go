package value

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type ScalarType int

func (typ ScalarType) String() string {
	switch typ {
	case String:
		return "string"
	case Bool:
		return "bool"
	case Number:
		return "number"
	}
	return "<unknown>"
}

const (
	String ScalarType = iota
	Bool
	Number
)

type Value interface {
	// Scalar tries to convert the value into a scalar value.
	Scalar() (z *Scalar, ok bool)

	// Array tries to convert the value into an array value.
	Array() (a *Array, ok bool)

	// Cmp returns an integer comparing two values (the receiver z and the
	// parameter v), and a boolean indicating whether it is possible to compare
	// the values using <, >, <= or >=. The result will be 0 if z == v, -1 if
	// z < v, and +1 if z > v.
	Cmp(v Value) (cmp int, ok bool)

	// String returns a string representation of the value.
	String() string

	// Len returns the length of the variable.
	Len() int

	// Encode encodes value to string in such a way that the resulting
	// string is a lexicographically correct representation of the
	// value.
	Encode() string
}

type Scalar struct {
	typ    ScalarType
	string string
	number float64
}

func NewNumber(f float64) *Scalar {
	return &Scalar{Number, "", f}
}

func NewString(s string) *Scalar {
	return &Scalar{String, s, 0}
}

func NewBool(b bool) *Scalar {
	n := .0
	if b {
		n = 1
	}
	return &Scalar{Bool, "", n}
}

func (z *Scalar) Scalar() (w *Scalar, ok bool) { return z, true }
func (z *Scalar) Array() (w *Array, ok bool)   { return nil, false }

func (z *Scalar) Type() ScalarType {
	return z.typ
}

func (z *Scalar) Cmp(w Value) (cmp int, ok bool) {
	v2, ok := w.Scalar()
	if !ok {
		return -1, false
	}
	if z.typ == v2.typ {
		return z.cmp(v2), true
	}
	// TODO: Don't use comparing numbers as a fallback.
	return z.Number().cmp(v2.Number()), true
}

func (z *Scalar) cmp(b *Scalar) int {
	switch z.typ {
	case String:
		return strings.Compare(z.string, b.string)
	case Number, Bool:
		if z.number < b.number {
			return -1
		} else if z.number > b.number {
			return 1
		}
		return 0
	}
	panic("unknown scalar type")
}

func (z *Scalar) Number() *Scalar {
	switch z.typ {
	case String:
		z.number, _ = strconv.ParseFloat(z.string, 64)
	}
	z.typ = Number
	return z
}

func (z *Scalar) Float64() float64 { return z.Number().number }
func (z *Scalar) Int() int         { return int(z.Number().number) }

func (z *Scalar) Bool() bool {
	cmp, _ := z.Cmp(NewBool(true))
	return cmp == 0
}

func (z *Scalar) String() string {
	switch z.typ {
	case String:
		return z.string
	case Number:
		return fmt.Sprintf("%.8g", z.number)
	case Bool:
		if z.number == 1 {
			return "true"
		}
		return "false"
	}
	panic("unknown scalar type")
}

func (z *Scalar) Encode() string {
	switch z.typ {
	case String:
		return strconv.Quote(z.string)
	default:
		return z.String()
	}
}

func (z *Scalar) Format(s fmt.State, verb rune) {
	var val interface{}
	switch verb {
	case 'v':
		fmt.Fprint(s, z.String())
		return
	case 'V':
		fmt.Fprint(s, z.Type())
		return
	// Boolean:
	case 't':
		val = z.Bool()
	// Integer:
	case 'b', 'c', 'd', 'o', 'U':
		// TODO: %b is different for integer and float.
		val = z.Int()
	// Floating-point:
	case 'e', 'E', 'f', 'F', 'g', 'G':
		val = z.Float64()
	// String:
	case 's':
		val = z.String()
	// Common for String and Integer
	case 'q', 'x', 'X':
		if z.typ == String {
			val = z.string
		} else {
			val = z.Int()
		}
	}
	fmt.Fprintf(s, formatVerb(s, verb), val)
}

func formatVerb(s fmt.State, verb rune) string {
	var buf bytes.Buffer
	buf.WriteRune('%')
	for _, c := range []int{' ', '0'} {
		if s.Flag(c) {
			buf.WriteRune(rune(c))
		}
	}
	if wid, ok := s.Width(); ok {
		fmt.Fprint(&buf, wid)
	}
	if prec, ok := s.Precision(); ok {
		fmt.Fprintf(&buf, ".%d", prec)
	}
	buf.WriteRune(verb)
	return buf.String()
}

func (z *Scalar) Len() int {
	return len(z.String())
}

func (z *Scalar) Add(x, y *Scalar) *Scalar {
	a, b := toFloat64(x, y)
	z.typ = Number
	z.number = a + b
	return z
}

func (z *Scalar) Sub(x, y *Scalar) *Scalar {
	a, b := toFloat64(x, y)
	z.typ = Number
	z.number = a - b
	return z
}

func (z *Scalar) Mul(x, y *Scalar) *Scalar {
	a, b := toFloat64(x, y)
	z.typ = Number
	z.number = a * b
	return z
}

func (z *Scalar) Div(x, y *Scalar) *Scalar {
	a, b := toFloat64(x, y)
	z.typ = Number
	z.number = a / b
	return z
}

func (z *Scalar) Mod(x, y *Scalar) *Scalar {
	a, b := toFloat64(x, y)
	z.typ = Number
	if int(b) == 0 {
		z.number = math.NaN()
	} else {
		z.number = float64(int(a) % int(b))
	}
	return z
}

func (z *Scalar) Neg(x *Scalar) *Scalar {
	z.typ = Number
	z.number = -x.Float64()
	return z
}

func toFloat64(x, y *Scalar) (float64, float64) {
	return x.Float64(), y.Float64()
}

func (z *Scalar) Concat(x, y *Scalar) *Scalar {
	z.typ = String
	z.string = x.String() + y.String()
	return z
}
