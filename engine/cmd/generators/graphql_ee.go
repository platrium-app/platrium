// cmd/generators/graphql_ee.go
package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	"github.com/99designs/gqlgen/plugin/resolvergen"
)

// Must match resolver.dir in gqlgen-ee.yml
const resolverDir = "internal/ee/graphql"

type eeOnly struct{ inner plugin.CodeGenerator }

func (eeOnly) Name() string { return "resolvergen" } // replaces the built-in

// isEEField is the filter: true only for fields defined in a schema file under /ee/.
// Introspection and other synthesized fields have no source position, so skip them.
func isEEField(f *codegen.Field) bool {
	if f == nil || f.FieldDefinition == nil || f.Position == nil || f.Position.Src == nil {
		return false
	}

	return strings.Contains(filepath.ToSlash(f.Position.Src.Name), "/ee/")
}

func (p eeOnly) GenerateCode(data *codegen.Data) error {
	d := *data
	d.Objects = nil
	for _, o := range data.Objects {
		oc := *o
		oc.Fields = nil
		for _, f := range o.Fields {
			if isEEField(f) {
				oc.Fields = append(oc.Fields, f)
			}
		}
		if len(oc.Fields) > 0 && oc.HasResolvers() {
			d.Objects = append(d.Objects, &oc)
		}
	}
	return p.inner.GenerateCode(&d)
}

func main() {
	cfg, err := config.LoadConfig("gqlgen-ee.yml")
	if err != nil {
		log.Fatal(err)
	}
	inner := resolvergen.New().(plugin.CodeGenerator)
	if err := api.Generate(cfg, api.ReplacePlugin(eeOnly{inner})); err != nil {
		log.Fatal(err)
	}
	if err := postProcess(resolverDir); err != nil {
		log.Fatal(err)
	}
}

// postProcess removes the `type xResolver struct{ *Resolver }` and
// `func (r *Resolver) X() ...` declarations gqlgen emits (they clash with the
// hand-written embedding structs) and adds the //go:build ee tag.
func postProcess(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.resolvers.go"))
	if err != nil {
		return err
	}
	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		type span struct{ start, end token.Pos }
		var removed []span
		kept := f.Decls[:0]
		for _, decl := range f.Decls {
			drop := false
			var doc *ast.CommentGroup
			switch d := decl.(type) {
			case *ast.GenDecl:
				drop = d.Tok == token.TYPE && isGeneratedResolverType(d)
				doc = d.Doc
			case *ast.FuncDecl:
				drop = recvName(d) == "Resolver" && d.Type.Params.NumFields() == 0
				doc = d.Doc
			}
			if drop {
				start := decl.Pos()
				if doc != nil {
					start = doc.Pos()
				}
				removed = append(removed, span{start, decl.End()})
				continue
			}
			kept = append(kept, decl)
		}
		f.Decls = kept

		// drop comment groups that belonged to removed declarations
		cs := f.Comments[:0]
		for _, cg := range f.Comments {
			gone := false
			for _, r := range removed {
				if cg.Pos() >= r.start && cg.End() <= r.end {
					gone = true
					break
				}
			}
			if !gone {
				cs = append(cs, cg)
			}
		}

		f.Comments = cs

		var buf bytes.Buffer
		if err := format.Node(&buf, fset, f); err != nil {
			return err
		}

		out := buf.Bytes()
		if !bytes.HasPrefix(out, []byte("//go:build ee")) {
			out = append([]byte("//go:build ee\n\n"), out...)
		}

		if err := os.WriteFile(path, out, 0o644); err != nil {
			return err
		}

		// If the file is empty after we stripped the structs, delete it!
		if len(f.Decls) == 0 {
			os.Remove(path)
		}
	}

	return nil
}

func isGeneratedResolverType(d *ast.GenDecl) bool {
	if len(d.Specs) == 0 {
		return false
	}
	for _, s := range d.Specs {
		ts, ok := s.(*ast.TypeSpec)
		if !ok {
			return false
		}
		n := ts.Name.Name
		if n == "Resolver" || !strings.HasSuffix(n, "Resolver") || !unicode.IsLower(rune(n[0])) {
			return false
		}
		if _, ok := ts.Type.(*ast.StructType); !ok {
			return false
		}
	}
	return true
}

func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
