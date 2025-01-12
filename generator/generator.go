package generator

import (
	"fmt"
	"html"
	"io"
	"reflect"
	"strings"

	"github.com/a-h/templ"
)

func NewRangeWriter(w io.Writer) *RangeWriter {
	return &RangeWriter{
		Current: templ.NewPosition(),
		w:       w,
	}
}

type RangeWriter struct {
	Current templ.Position
	w       io.Writer
}

func (rw *RangeWriter) WriteIndent(level int, s string) (r templ.Range, err error) {
	_, err = rw.Write(strings.Repeat("\t", level))
	if err != nil {
		return
	}
	return rw.Write(s)
}

func (rw *RangeWriter) Write(s string) (r templ.Range, err error) {
	r.From = templ.Position{
		Index: rw.Current.Index,
		Line:  rw.Current.Line,
		Col:   rw.Current.Col,
	}
	var n int
	for _, c := range s {
		if c == '\n' {
			rw.Current.Line++
			rw.Current.Col = 0
		}
		rw.Current.Col++
		n, err = io.WriteString(rw.w, string(c))
		rw.Current.Index += int64(n)
		if err != nil {
			return r, err
		}
	}
	r.To = rw.Current
	return r, err
}

func Generate(template templ.TemplateFile, w io.Writer) (sm *templ.SourceMap, err error) {
	g := generator{
		tf:        template,
		w:         NewRangeWriter(w),
		sourceMap: templ.NewSourceMap(),
	}
	err = g.generate()
	sm = g.sourceMap
	return
}

type generator struct {
	tf        templ.TemplateFile
	w         *RangeWriter
	sourceMap *templ.SourceMap
}

func (g *generator) generate() (err error) {
	if err = g.writeCodeGeneratedComment(); err != nil {
		return
	}
	if err = g.writePackage(); err != nil {
		return
	}
	if err = g.writeImports(); err != nil {
		return
	}
	if err = g.writeTemplates(); err != nil {
		return
	}
	return err
}

func (g *generator) writeCodeGeneratedComment() error {
	_, err := g.w.Write("// Code generated by templ DO NOT EDIT.\n\n")
	return err
}

func (g *generator) writePackage() error {
	var r templ.Range
	var err error
	// package
	if _, err = g.w.Write("package "); err != nil {
		return err
	}
	if r, err = g.w.Write(g.tf.Package.Expression.Value); err != nil {
		return err
	}
	g.sourceMap.Add(g.tf.Package.Expression, r)
	if _, err = g.w.Write("\n\n"); err != nil {
		return err
	}
	return nil
}

func (g *generator) writeImports() error {
	var r templ.Range
	var err error
	// Always import templ because it's the interface type of all templates.
	if _, err = g.w.Write("import \"github.com/a-h/templ\"\n"); err != nil {
		return err
	}
	// Always import context because it's the first parameter of a template function.
	if _, err = g.w.Write("import \"context\"\n"); err != nil {
		return err
	}
	// Always import io because it's the second parameter of a template function.
	if _, err = g.w.Write("import \"io\"\n"); err != nil {
		return err
	}
	for _, im := range g.tf.Imports {
		// import
		if _, err = g.w.Write("import "); err != nil {
			return err
		}
		if r, err = g.w.Write(im.Expression.Value); err != nil {
			return err
		}
		g.sourceMap.Add(im.Expression, r)
		if _, err = g.w.Write("\n"); err != nil {
			return err
		}
	}
	if _, err = g.w.Write("\n"); err != nil {
		return err
	}
	return nil
}

func (g *generator) writeTemplates() error {
	for i := 0; i < len(g.tf.Templates); i++ {
		if err := g.writeTemplate(g.tf.Templates[i]); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) writeTemplate(t templ.Template) error {
	var r templ.Range
	var err error
	var indentLevel int

	// Create thew new function.

	// func
	if _, err = g.w.Write("func "); err != nil {
		return err
	}
	if r, err = g.w.Write(t.Name.Value); err != nil {
		return err
	}
	g.sourceMap.Add(t.Name, r)
	// (ctx context.Context, w io.Writer,
	if _, err = g.w.Write("("); err != nil {
		return err
	}
	// Write parameters.
	if r, err = g.w.Write(t.Parameters.Value); err != nil {
		return err
	}
	g.sourceMap.Add(t.Parameters, r)
	// ) templ.Component {
	if _, err = g.w.Write(") templ.Component {\n"); err != nil {
		return err
	}
	indentLevel++
	// return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
	if _, err = g.w.WriteIndent(indentLevel, "return templ.ComponentFunc(func(ctx context.Context, w io.Writer) (err error) {\n"); err != nil {
		return err
	}
	{
		indentLevel++
		if err = g.writeNodes(indentLevel, t.Children); err != nil {
			return err
		}
		// return nil
		if _, err = g.w.WriteIndent(indentLevel, "return err\n"); err != nil {
			return err
		}
		indentLevel--
	}
	// })
	if _, err = g.w.WriteIndent(indentLevel, "})\n"); err != nil {
		return err
	}
	indentLevel--
	// }
	if _, err = g.w.WriteIndent(indentLevel, "}\n\n"); err != nil {
		return err
	}
	return nil
}

func (g *generator) writeNodes(indentLevel int, nodes []templ.Node) error {
	for i := 0; i < len(nodes); i++ {
		if err := g.writeNode(indentLevel, nodes[i]); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) writeNode(indentLevel int, node templ.Node) error {
	switch n := node.(type) {
	case templ.Element:
		g.writeElement(indentLevel, n)
	case templ.ForExpression:
		g.writeForExpression(indentLevel, n)
	case templ.CallTemplateExpression:
		g.writeCallTemplateExpression(indentLevel, n)
	case templ.IfExpression:
		g.writeIfExpression(indentLevel, n)
	case templ.SwitchExpression:
		g.writeSwitchExpression(indentLevel, n)
	case templ.StringExpression:
		g.writeStringExpression(indentLevel, n.Expression)
	case templ.Whitespace:
		// Whitespace is removed from template output to minify HTML.
	default:
		g.w.Write(fmt.Sprintf("Unhandled type: %v\n", reflect.TypeOf(n)))
	}
	return nil
}

func (g *generator) writeIfExpression(indentLevel int, n templ.IfExpression) error {
	var r templ.Range
	var err error
	// if
	if _, err = g.w.WriteIndent(indentLevel, `if `); err != nil {
		return err
	}
	// x == y
	if r, err = g.w.Write(n.Expression.Value); err != nil {
		return err
	}
	g.sourceMap.Add(n.Expression, r)
	// Then.
	// {
	if _, err = g.w.Write(` {` + "\n"); err != nil {
		return err
	}
	indentLevel++
	g.writeNodes(indentLevel, n.Then)
	indentLevel--
	if len(n.Else) > 0 {
		// } else {
		if _, err = g.w.WriteIndent(indentLevel, `} else {`+"\n"); err != nil {
			return err
		}
		indentLevel++
		g.writeNodes(indentLevel, n.Else)
		indentLevel--
	}
	// }
	if _, err = g.w.WriteIndent(indentLevel, `}`+"\n"); err != nil {
		return err
	}
	return nil
}

func (g *generator) writeSwitchExpression(indentLevel int, n templ.SwitchExpression) error {
	var r templ.Range
	var err error
	// switch
	if _, err = g.w.WriteIndent(indentLevel, `switch `); err != nil {
		return err
	}
	// val
	if r, err = g.w.Write(n.Expression.Value); err != nil {
		return err
	}
	g.sourceMap.Add(n.Expression, r)
	// {
	if _, err = g.w.Write(` {` + "\n"); err != nil {
		return err
	}

	if len(n.Cases) > 0 {
		for _, c := range n.Cases {
			// case
			if _, err = g.w.WriteIndent(indentLevel, `case `); err != nil {
				return err
			}
			//val
			if r, err = g.w.Write(c.Expression.Value); err != nil {
				return err
			}
			g.sourceMap.Add(c.Expression, r)
			if _, err = g.w.Write(`:` + "\n"); err != nil {
				return err
			}
			indentLevel++
			g.writeNodes(indentLevel, c.Children)
			indentLevel--
		}
	}

	if len(n.Default) > 0 {
		if _, err = g.w.WriteIndent(indentLevel, `default:`); err != nil {
			return err
		}
		indentLevel++
		g.writeNodes(indentLevel, n.Default)
		indentLevel--
	}
	// }
	if _, err = g.w.WriteIndent(indentLevel, `}`+"\n"); err != nil {
		return err
	}
	return nil
}

func (g *generator) writeCallTemplateExpression(indentLevel int, n templ.CallTemplateExpression) error {
	var r templ.Range
	var err error
	if r, err = g.w.WriteIndent(indentLevel, `err = `); err != nil {
		return err
	}
	// Template expression.
	if r, err = g.w.Write(fmt.Sprintf(`%s`, n.Expression.Value)); err != nil {
		return err
	}
	g.sourceMap.Add(n.Expression, r)
	// .Render(ctx w)
	if _, err = g.w.Write(".Render(ctx, w)\n"); err != nil {
		return err
	}
	if err = g.writeErrorHandler(indentLevel); err != nil {
		return err
	}
	return nil
}

func (g *generator) writeForExpression(indentLevel int, n templ.ForExpression) error {
	var r templ.Range
	var err error
	// if
	if _, err = g.w.WriteIndent(indentLevel, `for `); err != nil {
		return err
	}
	// i, v := range p.Stuff
	if r, err = g.w.Write(n.Expression.Value); err != nil {
		return err
	}
	g.sourceMap.Add(n.Expression, r)
	// {
	if _, err = g.w.Write(` {` + "\n"); err != nil {
		return err
	}
	// Children.
	indentLevel++
	g.writeNodes(indentLevel, n.Children)
	indentLevel--
	// }
	if _, err = g.w.WriteIndent(indentLevel, `}`+"\n"); err != nil {
		return err
	}
	return nil
}

func (g *generator) writeErrorHandler(indentLevel int) (err error) {
	_, err = g.w.WriteIndent(indentLevel, "if err != nil {\n")
	if err != nil {
		return err
	}
	indentLevel++
	_, err = g.w.WriteIndent(indentLevel, "return err\n")
	if err != nil {
		return err
	}
	indentLevel--
	_, err = g.w.WriteIndent(indentLevel, "}\n")
	if err != nil {
		return err
	}
	return err
}

func (g *generator) writeElement(indentLevel int, n templ.Element) error {
	if n.IsVoidElement() {
		return g.writeVoidElement(indentLevel, n)
	}
	return g.writeStandardElement(indentLevel, n)
}

func (g *generator) writeVoidElement(indentLevel int, n templ.Element) (err error) {
	if len(n.Children) > 0 {
		return fmt.Errorf("writeVoidElement: void element %q must not have child elements", n.Name)
	}
	if len(n.Attributes) == 0 {
		// <div/>
		if _, err = g.w.WriteIndent(indentLevel, fmt.Sprintf(`_, err = io.WriteString(w, "<%s/>")`+"\n", html.EscapeString(n.Name))); err != nil {
			return err
		}
		if err = g.writeErrorHandler(indentLevel); err != nil {
			return err
		}
	} else {
		// <div
		if _, err = g.w.WriteIndent(indentLevel, fmt.Sprintf(`_, err = io.WriteString(w, "<%s")`+"\n", html.EscapeString(n.Name))); err != nil {
			return err
		}
		if err = g.writeErrorHandler(indentLevel); err != nil {
			return err
		}
		if err = g.writeElementAttributes(indentLevel, n); err != nil {
			return err
		}
		// />
		if _, err = g.w.WriteIndent(indentLevel, `_, err = io.WriteString(w, "/>")`+"\n"); err != nil {
			return err
		}
		if err = g.writeErrorHandler(indentLevel); err != nil {
			return err
		}
	}
	return err
}

func (g *generator) writeStandardElement(indentLevel int, n templ.Element) (err error) {
	if len(n.Attributes) == 0 {
		// <div>
		if _, err = g.w.WriteIndent(indentLevel, fmt.Sprintf(`_, err = io.WriteString(w, "<%s>")`+"\n", html.EscapeString(n.Name))); err != nil {
			return err
		}
		if err = g.writeErrorHandler(indentLevel); err != nil {
			return err
		}
	} else {
		// <div
		if _, err = g.w.WriteIndent(indentLevel, fmt.Sprintf(`_, err = io.WriteString(w, "<%s")`+"\n", html.EscapeString(n.Name))); err != nil {
			return err
		}
		if err = g.writeErrorHandler(indentLevel); err != nil {
			return err
		}
		if err = g.writeElementAttributes(indentLevel, n); err != nil {
			return err
		}
		// >
		if _, err = g.w.WriteIndent(indentLevel, `_, err = io.WriteString(w, ">")`+"\n"); err != nil {
			return err
		}
		if err = g.writeErrorHandler(indentLevel); err != nil {
			return err
		}
	}
	// Children.
	g.writeNodes(indentLevel, n.Children)
	// </div>
	if _, err = g.w.WriteIndent(indentLevel, fmt.Sprintf(`_, err = io.WriteString(w, "</%s>")`+"\n", html.EscapeString(n.Name))); err != nil {
		return err
	}
	if err = g.writeErrorHandler(indentLevel); err != nil {
		return err
	}
	return err
}

func (g *generator) writeElementAttributes(indentLevel int, n templ.Element) (err error) {
	var r templ.Range
	for i := 0; i < len(n.Attributes); i++ {
		switch attr := n.Attributes[i].(type) {
		case templ.ConstantAttribute:
			name := html.EscapeString(attr.Name)
			value := html.EscapeString(attr.Value)
			if _, err = g.w.WriteIndent(indentLevel, fmt.Sprintf(`_, err = io.WriteString(w, " %s=\"%s\"")`+"\n", name, value)); err != nil {
				return err
			}
			if err = g.writeErrorHandler(indentLevel); err != nil {
				return err
			}
		case templ.ExpressionAttribute:
			name := html.EscapeString(attr.Name)
			// Name
			if _, err = g.w.WriteIndent(indentLevel, fmt.Sprintf(`_, err = io.WriteString(w, " %s=")`+"\n", name)); err != nil {
				return err
			}
			if err = g.writeErrorHandler(indentLevel); err != nil {
				return err
			}
			// Value.
			// Open quote.
			if _, err = g.w.WriteIndent(indentLevel, `_, err = io.WriteString(w, "\"")`+"\n"); err != nil {
				return err
			}
			if err = g.writeErrorHandler(indentLevel); err != nil {
				return err
			}
			// io.WriteString(w, templ.EscapeString(
			if _, err = g.w.WriteIndent(indentLevel, "_, err = io.WriteString(w, templ.EscapeString("); err != nil {
				return err
			}
			// p.Name()
			if r, err = g.w.Write(attr.Expression.Value); err != nil {
				return err
			}
			g.sourceMap.Add(attr.Expression, r)
			// ))
			if _, err = g.w.Write("))\n"); err != nil {
				return err
			}
			if err = g.writeErrorHandler(indentLevel); err != nil {
				return err
			}
			// Close quote.
			if _, err = g.w.WriteIndent(indentLevel, `_, err = io.WriteString(w, "\"")`+"\n"); err != nil {
				return err
			}
			if err = g.writeErrorHandler(indentLevel); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown attribute type %s", reflect.TypeOf(n.Attributes[i]))
		}
	}
	return err
}

func (g *generator) writeStringExpression(indentLevel int, e templ.Expression) error {
	var r templ.Range
	var err error
	// io.WriteString(w, templ.EscapeString(
	if _, err = g.w.WriteIndent(indentLevel, "_, err = io.WriteString(w, templ.EscapeString("); err != nil {
		return err
	}
	// p.Name()
	if r, err = g.w.Write(e.Value); err != nil {
		return err
	}
	g.sourceMap.Add(e, r)
	// ))
	if _, err = g.w.Write("))\n"); err != nil {
		return err
	}
	if err = g.writeErrorHandler(indentLevel); err != nil {
		return err
	}
	return nil
}
