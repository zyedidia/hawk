package compiler

import (
	"bufio"
	"io"

	"github.com/mibk/hawk/scan"
	"github.com/mibk/hawk/value"
)

type Decl interface{}

type Scope interface {
	Var(name string) *value.Value
	SetVar(name string, v *value.Value)
}

type Program struct {
	sc       *scan.Scanner
	begin    []Stmt
	pActions []Stmt
	end      []Stmt

	vars   map[string]*value.Value
	funcs  map[string]FuncDecl
	retval *value.Value
}

func NewProgram(sc *scan.Scanner) *Program {
	return &Program{
		sc:    sc,
		vars:  make(map[string]*value.Value),
		funcs: make(map[string]FuncDecl),
	}
}

func (p Program) Var(name string) *value.Value {
	if v, ok := p.vars[name]; ok {
		return v
	}
	// Global "magic" variables.
	switch name {
	case "NF":
		return value.NewNumber(float64(p.sc.NF()))
	}
	return new(value.Value)
}

func (p Program) SetVar(name string, v *value.Value) {
	p.vars[name] = v
}

func (p Program) Run(in io.Reader) {
	p.Begin(p.sc.Writer)
	if p.anyPatternActions() {
		in := bufio.NewReader(in)
		for {
			line, err := in.ReadBytes('\n')
			if err != nil {
				break
			}
			p.sc.SplitLine(string(line))
			p.Exec(p.sc.Writer)
		}
		p.End(p.sc.Writer)
	}
}

func (p Program) Begin(w io.Writer) {
	for _, a := range p.begin {
		a.Exec(w)
	}
}

func (p Program) End(w io.Writer) {
	for _, a := range p.end {
		a.Exec(w)
	}
}

func (p Program) anyPatternActions() bool {
	return len(p.pActions) > 0 || len(p.end) > 0
}

func (p Program) Exec(w io.Writer) {
	for _, a := range p.pActions {
		a.Exec(w)
	}
}

type BeginAction struct {
	Stmt
}

type EndAction struct {
	Stmt
}

type PatternAction struct {
	pattern Expr
	action  Stmt
}

func (p PatternAction) Exec(w io.Writer) Status {
	if p.pattern == nil || p.pattern.Eval(w).Bool() {
		p.action.Exec(w)
	}
	return StatusNone
}

type FuncDecl struct {
	scope *FuncScope
	name  string
	args  []string
	body  Stmt
}

type FuncScope struct {
	stack []map[string]*value.Value
}

func (f *FuncScope) Push() {
	f.stack = append(f.stack, make(map[string]*value.Value))
}

func (f *FuncScope) Pull() {
	f.stack = f.stack[:len(f.stack)-1]
}

func (f *FuncScope) Var(name string) *value.Value {
	v, ok := f.currScope()[name]
	if !ok {
		return new(value.Value)
	}
	return v
}

func (f *FuncScope) SetVar(name string, v *value.Value) {
	f.currScope()[name] = v
}

func (f *FuncScope) currScope() map[string]*value.Value {
	if f.stack == nil {
		panic("stack shouldn't be nil")
	}
	return f.stack[len(f.stack)-1]
}
