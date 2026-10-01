package patterns

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

// sizeConstEnv resolves package-level constants of internal/patterns and
// internal/tokens from source, so a test can evaluate the constant a pattern
// passes as a default font size and see which named constants it went
// through.
type sizeConstEnv struct {
	pkgs map[string]map[string]ast.Expr // package -> const name -> value expr
}

func parseConstDecls(t *testing.T, fset *token.FileSet, dir string) (map[string]ast.Expr, []*ast.File) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	consts := map[string]ast.Expr{}
	var files []*ast.File
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				if len(vs.Values) != len(vs.Names) {
					continue // iota / implicit repetition: not a size
				}
				for i, name := range vs.Names {
					consts[name.Name] = vs.Values[i]
				}
			}
		}
	}
	return consts, files
}

// eval returns the constant value of e (resolved in package pkg) and the
// named constants the resolution passed through; ok is false when e is not a
// constant expression this evaluator understands (a variable, a field, a call).
func (env *sizeConstEnv) eval(pkg string, e ast.Expr) (v constant.Value, chain []string, ok bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		return constant.MakeFromLiteral(x.Value, x.Kind, 0), nil, true
	case *ast.ParenExpr:
		return env.eval(pkg, x.X)
	case *ast.Ident:
		def, found := env.pkgs[pkg][x.Name]
		if !found {
			return nil, nil, false
		}
		v, chain, ok = env.eval(pkg, def)
		return v, append([]string{x.Name}, chain...), ok
	case *ast.SelectorExpr:
		id, isIdent := x.X.(*ast.Ident)
		if !isIdent || env.pkgs[id.Name] == nil {
			return nil, nil, false
		}
		return env.eval(id.Name, x.Sel)
	case *ast.BinaryExpr:
		a, ca, okA := env.eval(pkg, x.X)
		b, cb, okB := env.eval(pkg, x.Y)
		if !okA || !okB {
			return nil, nil, false
		}
		return constant.BinaryOp(a, x.Op, b), append(ca, cb...), true
	case *ast.CallExpr:
		if fn, isIdent := x.Fun.(*ast.Ident); isIdent && fn.Name == "float64" && len(x.Args) == 1 {
			return env.eval(pkg, x.Args[0])
		}
	}
	return nil, nil, false
}

func constPt(v constant.Value) float64 {
	f, _ := constant.Float64Val(constant.ToFloat(v))
	return f
}

func onTypeScalePt(pt float64) bool {
	return tokens.OnTypeScaleHPt(int(math.Round(pt * 100)))
}

func isNumericLit(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && (lit.Kind == token.INT || lit.Kind == token.FLOAT)
}

// textSizeKeys are the composite-literal fields and variables that carry a
// text size in pattern code.
func isTextSizeName(name string) bool {
	switch name {
	case "Size", "SizePt", "sizePt", "size":
		return true
	}
	return strings.HasSuffix(name, "Size") || strings.HasSuffix(name, "SizePt")
}

// TestPatternDefaultSizesOnTypeScale is go-slide-creator-vmdfm's guard: every
// default font size a pattern resolves names a constant (no numeric literal
// as a ResolveSize fallback, a paragraph size or a size variable), and every
// such constant is on the type scale or is an allow-listed off-scale size
// (offScaleDefaultReasons) whose reason is recorded.
func TestPatternDefaultSizesOnTypeScale(t *testing.T) {
	fset := token.NewFileSet()
	patternConsts, files := parseConstDecls(t, fset, ".")
	tokenConsts, _ := parseConstDecls(t, fset, filepath.Join("..", "tokens"))
	env := &sizeConstEnv{pkgs: map[string]map[string]ast.Expr{"patterns": patternConsts, "tokens": tokenConsts}}

	var problems []string
	report := func(n ast.Node, format string, args ...any) {
		args = append([]any{fset.Position(n.Pos())}, args...)
		problems = append(problems, fmt.Sprintf("%s: "+format, args...))
	}
	checkNamed := func(n ast.Node, e ast.Expr, what string) {
		v, chain, ok := env.eval("patterns", e)
		if !ok || v.Kind() == constant.Unknown {
			return // not a constant default (a measured or derived size)
		}
		pt := constPt(v)
		if onTypeScalePt(pt) {
			return
		}
		for _, name := range chain {
			if _, listed := offScaleDefaultReasons[name]; listed {
				return
			}
		}
		report(n, "%s %v (%.4gpt) is off the type scale and not allow-listed in offScaleDefaultReasons", what, chain, pt)
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if fn, ok := x.Fun.(*ast.Ident); ok && fn.Name == "ResolveSize" && len(x.Args) == 2 {
					if isNumericLit(x.Args[1]) {
						report(x, "ResolveSize default is the literal %s; name a type-scale constant (type_scale.go)", x.Args[1].(*ast.BasicLit).Value)
					} else {
						checkNamed(x, x.Args[1], "ResolveSize default")
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := x.Key.(*ast.Ident); ok && isTextSizeName(key.Name) && isNumericLit(x.Value) {
					report(x, "text size %s: %s is a literal; name a type-scale constant", key.Name, x.Value.(*ast.BasicLit).Value)
				}
			case *ast.AssignStmt:
				if len(x.Lhs) == 1 && len(x.Rhs) == 1 && isNumericLit(x.Rhs[0]) {
					if id, ok := x.Lhs[0].(*ast.Ident); ok && isTextSizeName(id.Name) {
						report(x, "text size %s = %s is a literal; name a type-scale constant", id.Name, x.Rhs[0].(*ast.BasicLit).Value)
					}
				}
			}
			return true
		})
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}

	// The allow-list carries no stale entries: each names an off-scale
	// constant, with a reason.
	for name, reason := range offScaleDefaultReasons {
		def, ok := patternConsts[name]
		if !ok {
			t.Errorf("offScaleDefaultReasons lists %q, which is not a constant of this package", name)
			continue
		}
		if v, _, ok := env.eval("patterns", def); ok && onTypeScalePt(constPt(v)) {
			t.Errorf("offScaleDefaultReasons lists %q, but %.4gpt is on the type scale; drop the entry", name, constPt(v))
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("offScaleDefaultReasons[%q] has no reason", name)
		}
	}
	// The named scale steps are what they claim to be.
	for name, pt := range map[string]float64{
		"scaleDisplayPt": scaleDisplayPt, "scaleLeadPt": scaleLeadPt, "scaleSubheadPt": scaleSubheadPt,
		"scaleBodyPt": scaleBodyPt, "scaleDenseBodyPt": scaleDenseBodyPt, "scaleCaptionPt": scaleCaptionPt, "scaleKPIPt": scaleKPIPt,
	} {
		if !onTypeScalePt(pt) {
			t.Errorf("%s = %.4gpt is not a type-scale step", name, pt)
		}
	}
}

// isLadderName reports a variable that holds a fit ladder: the sizes a
// pattern steps through while measuring (fooSteps, fooScales, heroSizes, …).
func isLadderName(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{"steps", "scales", "scale", "sizes"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// isNonSizeField reports a ladder struct field that is geometry, not a text
// size (a gap, padding, a percentage or fraction of the area, a weight).
func isNonSizeField(name string) bool {
	lower := strings.ToLower(name)
	for _, part := range []string{"gap", "pad", "pct", "frac", "weight", "inset", "width", "limit"} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

// structFieldIndex maps struct type names to their ordered field names, per
// scope: the package (key "") and each function that declares local types.
type structFieldIndex map[string]map[string][]string

func indexStructFields(files []*ast.File) structFieldIndex {
	idx := structFieldIndex{"": {}}
	add := func(scope string, ts *ast.TypeSpec) {
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return
		}
		var names []string
		for _, f := range st.Fields.List {
			for _, n := range f.Names {
				names = append(names, n.Name)
			}
		}
		if idx[scope] == nil {
			idx[scope] = map[string][]string{}
		}
		idx[scope][ts.Name.Name] = names
	}
	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						add("", ts)
					}
				}
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				ast.Inspect(d.Body, func(n ast.Node) bool {
					if ts, ok := n.(*ast.TypeSpec); ok {
						add(d.Name.Name, ts)
					}
					return true
				})
			}
		}
	}
	return idx
}

func (idx structFieldIndex) fields(scope, name string) ([]string, bool) {
	if f, ok := idx[scope][name]; ok {
		return f, true
	}
	f, ok := idx[""][name]
	return f, ok
}

// TestPatternFitLaddersOnTypeScale extends the type-scale guard to the shrink
// ladders patterns walk while measuring a fit (go-slide-creator-vmdfm): every
// text size in a ladder — a composite literal assigned or appended to a
// *steps / *scales / *sizes variable — names a constant (no numeric literal)
// that is a type-scale step, an allow-listed default (offScaleDefaultReasons)
// or an allow-listed ladder step (offScaleLadderReasons). Geometry fields of a
// ladder struct (gaps, padding, percentages) are not text sizes.
func TestPatternFitLaddersOnTypeScale(t *testing.T) {
	fset := token.NewFileSet()
	patternConsts, files := parseConstDecls(t, fset, ".")
	tokenConsts, _ := parseConstDecls(t, fset, filepath.Join("..", "tokens"))
	shapegridConsts, _ := parseConstDecls(t, fset, filepath.Join("..", "shapegrid"))
	env := &sizeConstEnv{pkgs: map[string]map[string]ast.Expr{
		"patterns": patternConsts, "tokens": tokenConsts, "shapegrid": shapegridConsts,
	}}
	structs := indexStructFields(files)

	var problems []string
	ladders := 0
	report := func(n ast.Node, format string, args ...any) {
		args = append([]any{fset.Position(n.Pos())}, args...)
		problems = append(problems, fmt.Sprintf("%s: "+format, args...))
	}
	checkLeaf := func(ladder string, e ast.Expr) {
		if isNumericLit(e) {
			report(e, "ladder %s: size %s is a literal; name a type-scale step (type_scale.go)", ladder, e.(*ast.BasicLit).Value)
			return
		}
		v, chain, ok := env.eval("patterns", e)
		if !ok || v.Kind() == constant.Unknown {
			return // measured, authored or derived size
		}
		pt := constPt(v)
		if onTypeScalePt(pt) {
			return
		}
		// An allow-listed constant passes as itself, not as the operand of
		// arithmetic that lands somewhere else off the scale.
		if _, derived := ast.Unparen(e).(*ast.BinaryExpr); !derived {
			for _, name := range chain {
				_, isDefault := offScaleDefaultReasons[name]
				_, isLadder := offScaleLadderReasons[name]
				if isDefault || isLadder {
					return
				}
			}
		}
		report(e, "ladder %s: step %v (%.4gpt) is off the type scale and not allow-listed in offScaleLadderReasons", ladder, chain, pt)
	}
	var walk func(scope, ladder string, lit *ast.CompositeLit, typ ast.Expr)
	walk = func(scope, ladder string, lit *ast.CompositeLit, typ ast.Expr) {
		if lit.Type != nil {
			typ = lit.Type
		}
		switch tt := typ.(type) {
		case *ast.ArrayType:
			for _, el := range lit.Elts {
				if c, ok := el.(*ast.CompositeLit); ok {
					walk(scope, ladder, c, tt.Elt)
				} else if id, ok := tt.Elt.(*ast.Ident); ok && id.Name == "float64" {
					checkLeaf(ladder, el)
				}
			}
		case *ast.Ident:
			fields, ok := structs.fields(scope, tt.Name)
			if !ok {
				return
			}
			for i, el := range lit.Elts {
				name := ""
				if kv, isKV := el.(*ast.KeyValueExpr); isKV {
					if k, isIdent := kv.Key.(*ast.Ident); isIdent {
						name = k.Name
					}
					el = kv.Value
				} else if i < len(fields) {
					name = fields[i]
				}
				if name == "" || isNonSizeField(name) {
					continue
				}
				checkLeaf(ladder, el)
			}
		}
	}
	visitRHS := func(scope, ladder string, rhs ast.Expr) {
		switch r := rhs.(type) {
		case *ast.CompositeLit:
			ladders++
			walk(scope, ladder, r, nil)
		case *ast.CallExpr:
			fn, ok := r.Fun.(*ast.Ident)
			if !ok || fn.Name != "append" || len(r.Args) == 0 {
				return
			}
			for _, arg := range r.Args {
				if c, ok := arg.(*ast.CompositeLit); ok {
					ladders++
					walk(scope, ladder, c, nil)
				}
			}
		}
	}
	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, name := range vs.Names {
						if i < len(vs.Values) && isLadderName(name.Name) {
							visitRHS("", name.Name, vs.Values[i])
						}
					}
				}
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				scope := d.Name.Name
				ast.Inspect(d.Body, func(n ast.Node) bool {
					as, ok := n.(*ast.AssignStmt)
					if !ok || len(as.Lhs) != len(as.Rhs) {
						return true
					}
					for i, lhs := range as.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && isLadderName(id.Name) {
							visitRHS(scope, id.Name, as.Rhs[i])
						}
					}
					return true
				})
			}
		}
	}
	if ladders < 10 {
		t.Fatalf("found only %d fit ladders; the source scan has lost track of them", ladders)
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
	for name, reason := range offScaleLadderReasons {
		def, ok := patternConsts[name]
		if !ok {
			t.Errorf("offScaleLadderReasons lists %q, which is not a constant of this package", name)
			continue
		}
		if v, _, ok := env.eval("patterns", def); ok && onTypeScalePt(constPt(v)) {
			t.Errorf("offScaleLadderReasons lists %q, but %.4gpt is on the type scale; drop the entry", name, constPt(v))
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("offScaleLadderReasons[%q] has no reason", name)
		}
	}
}
