package gobundler

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func Bundle(patterns ...string) (string, error) {
	pkg, err := NewPackage(patterns...)
	if err != nil {
		return "", err
	}

	isRetain := func(pkg *packages.Package) bool {
		// always unfold given package
		if pkg.PkgPath == "command-line-arguments" {
			return false
		}
		// std
		if isStandardImportPath(pkg.PkgPath) {
			return true
		}
		return false
	}
	err = pkg.scan(isRetain)
	if err != nil {
		return "", err
	}

	var dstPkgName string
	if len(pkg.Roots) == 1 {
		if pkg.Roots[0].PkgPath == "command-line-arguments" {
			dstPkgName = "main" // FIXME
		} else {
			dstPkgName = GetPackageNameFromPath(pkg.Roots[0].PkgPath)
		}
	} else {
		if pkg.Roots[0].PkgPath == "command-line-arguments" {
			dstPkgName = "main" // FIXME
			// go/packages.Load ensures that if one of multiple is file path, all others are file paths under same directory.
			// see: https://github.com/golang/tools/blob/f12a0aeabe48cd7a30bfa4c59ca57747b54628e1/go/packages/golist.go#L941
		} else {
			dstPkgName = "bundled" // FIXME
		}
	}

	out, err := pkg.unfold(dstPkgName)
	if err != nil {
		return "", err
	}
	return out, nil
}

type PkgPath string

type Packages struct {
	Roots []*packages.Package

	// All following fields are calculated
	Unfold        map[PkgPath]*packages.Package
	ImportsUnfold []PkgPath // in topological order
	ImportsRetain []PkgPath // in topological order
}

func NewPackage(patterns ...string) (*Packages, error) {
	// load
	config := &packages.Config{
		Mode: packages.LoadAllSyntax,
		// Env:  os.Environ(), // specify custom env if needed: like GOOS, GOARCH, etc.
	}
	pkgs, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, err
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("package load errors: %v", pkg.Errors)
		}
	}
	return &Packages{
		Roots: pkgs,
	}, nil
}

func (p *Packages) scan(isRetain func(*packages.Package) bool) error {
	var (
		unfold        = make(map[PkgPath]*packages.Package)
		retain        = make(map[PkgPath]struct{})
		importsUnfold []PkgPath
		importsRetain []PkgPath
	)

	var walkImports func(pkg *packages.Package)
	walkImports = func(pkg *packages.Package) {
		// sort for replicability
		imps := make([]*packages.Package, 0, len(pkg.Imports))
		for _, imp := range pkg.Imports {
			imps = append(imps, imp)
		}
		sort.Slice(imps, func(i, j int) bool {
			return imps[i].PkgPath < imps[j].PkgPath
		})

		for _, imp := range imps {
			if isRetain(imp) {
				if _, ok := retain[PkgPath(imp.PkgPath)]; !ok {
					retain[PkgPath(imp.PkgPath)] = struct{}{}
					importsRetain = append(importsRetain, PkgPath(imp.PkgPath))
				}
				continue
			}
			walkImports(imp)
		}
		if _, ok := unfold[PkgPath(pkg.PkgPath)]; !ok {
			unfold[PkgPath(pkg.PkgPath)] = pkg
			importsUnfold = append(importsUnfold, PkgPath(pkg.PkgPath))
		}
	}
	for _, root := range p.Roots {
		walkImports(root)
	}

	p.Unfold = unfold
	p.ImportsUnfold = importsUnfold
	p.ImportsRetain = importsRetain

	return nil
}

func (p *Packages) unfold(dstPkgName string) (string, error) {
	pkgUnfoldStrFn := func(pkgPath PkgPath) string {
		for i, imp := range p.ImportsUnfold {
			if imp == pkgPath {
				return fmt.Sprintf("unfold%d", i)
			}
		}
		return ""
	}
	pkgRetainStrFn := func(pkgPath PkgPath) string {
		for i, imp := range p.ImportsRetain {
			if imp == pkgPath {
				return fmt.Sprintf("retain%d", i)
			}
		}
		return ""
	}

	// comment map
	cmaps := make(map[PkgPath]map[*ast.File]ast.CommentMap)
	for pkgPath, pkg := range p.Unfold {
		cmaps[pkgPath] = make(map[*ast.File]ast.CommentMap)
		for _, file := range pkg.Syntax {
			cmaps[pkgPath][file] = ast.NewCommentMap(pkg.Fset, file, file.Comments)
		}
	}

	// edit Unfold Packages
	for _, pkgPath := range p.ImportsUnfold {
		pkg := p.Unfold[pkgPath]
		for _, file := range pkg.Syntax {
			astutil.Apply(file, func(c *astutil.Cursor) bool {
				update := false
				update = replaceQualifiedIdent(c, pkg, pkgUnfoldStrFn, pkgRetainStrFn)
				if update {
					return true
				}
				if pkgPath != "command-line-arguments" {
					update = replaceTopLevel(c, pkg, pkgUnfoldStrFn)
					if update {
						return true
					}
				}
				update = replaceFieldSelector(c, pkg, pkgUnfoldStrFn)
				if update {
					return true
				}
				return true
			}, nil)
		}
	}

	// delete import statements
	for _, pkgPath := range p.ImportsUnfold {
		pkg := p.Unfold[pkgPath]
		for _, f := range pkg.Syntax {
			newDecls := []ast.Decl{}
			for _, decl := range f.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.IMPORT {
					newDecls = append(newDecls, decl)
				}
			}
			f.Decls = newDecls
		}
	}

	// filter comments
	for pkgPath, pkg := range p.Unfold {
		for _, file := range pkg.Syntax {
			file.Comments = cmaps[pkgPath][file].Filter(file).Comments()
		}
	}

	// output
	pw := NewPkgWriter()

	pw.WritePackageClause(dstPkgName)

	pw.WriteImportDecl(p.ImportsRetain, pkgRetainStrFn)

	for _, pkgPath := range p.ImportsUnfold {
		pkg := p.Unfold[pkgPath]
		err := pw.WritePackage(pkg.Fset, pkg.Syntax, string(pkgPath))
		if err != nil {
			return "", err
		}
	}

	return pw.String(), nil
}

// GetPackageIdentifier は golang/go の慣習に則り
// ImportSpec から利用可能なパッケージ名を導出します。
func GetPackageIdentifier(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}

	pkgPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return ""
	}

	return GetPackageNameFromPath(pkgPath)
}

func GetPackageNameFromPath(pkgPath string) string {
	pkgPath, _, ok := module.SplitPathVersion(pkgPath)
	if !ok {
		return ""
	}
	return path.Base(pkgPath)
}

const Delim = "__"

func replaceQualifiedIdent(cursor *astutil.Cursor, pkg *packages.Package, pkgUnfoldStrFn, pkgRetainStrFn func(PkgPath) string) (update bool) {
	node := cursor.Node()

	sel, ok := node.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	pkgPath := isPackageIdent(pkg, ident)
	if pkgPath == "" {
		return false
	}

	pkgUnfoldStr := pkgUnfoldStrFn(PkgPath(pkgPath))
	if pkgUnfoldStr != "" {
		name := fmt.Sprintf("%s%s%s", pkgUnfoldStr, Delim, sel.Sel.Name)
		newIdent := &ast.Ident{
			Name:    name,
			NamePos: sel.Sel.NamePos,
		}

		cursor.Replace(newIdent)
		return true
	}

	pkgRetainStr := pkgRetainStrFn(PkgPath(pkgPath))
	newSel := &ast.SelectorExpr{
		X: &ast.Ident{
			Name:    pkgRetainStr,
			NamePos: ident.NamePos,
		},
		Sel: &ast.Ident{
			Name:    sel.Sel.Name,
			NamePos: sel.Sel.NamePos,
		},
	}

	cursor.Replace(newSel)
	return true
}

func replaceFieldSelector(cursor *astutil.Cursor, pkg *packages.Package, pkgUnfoldStrFn func(PkgPath) string) (update bool) {
	node := cursor.Node()

	ident, ok := node.(*ast.Ident)
	if !ok {
		return false
	}

	obj := pkg.TypesInfo.Uses[ident]
	if obj == nil {
		return false
	}
	if obj.Pkg() == nil {
		return false
	}

	vr, ok := obj.(*types.Var)
	if !ok || !vr.Embedded() || vr.Kind() != types.FieldVar {
		return false
	}

	// ok, it's embedded field.

	typ := obj.Type()
	if deref, ok := typ.(*types.Pointer); ok {
		typ = deref.Elem()
	}

	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	originPkg := named.Obj().Pkg()
	if originPkg == nil {
		panic("originPkg is nil: " + fmt.Sprintf("%s, %s", ident.Name, named.String())) // builtin types, like error
	}

	pkgUnfoldStr := pkgUnfoldStrFn(PkgPath(originPkg.Path()))
	if pkgUnfoldStr == "" {
		return false
	}

	newIdent := &ast.Ident{
		Name:    fmt.Sprintf("%s%s%s", pkgUnfoldStr, Delim, ident.Name),
		NamePos: ident.NamePos,
	}
	cursor.Replace(newIdent)
	return true
}

// 与えられたオブジェクトがトップレベルで宣言された識別子かどうかを返す
func isTopLevelIdent(obj types.Object, isPkgName bool) bool {
	if obj == nil {
		return false // see .Defs doc
	}

	var packages *types.Scope = nil
	if isPkgName {
		if obj.Parent() == nil { // should be file scope
			return false
		}
		packages = obj.Parent().Parent()
	} else {
		packages = obj.Parent()
	}

	if packages == nil { // should be package scope
		return false
	}

	universe := packages.Parent()
	// FIXME: universe scope の判定がこれで完全かは不明
	if universe == nil || universe.Lookup("panic") == nil { // should be universe scope
		return false
	}

	return universe.Parent() == nil // universe should not have parent
}

// パッケージシンボルならパッケージのパスを返す
// そうでなければ、空文字
func isPackageIdent(pkg *packages.Package, ident *ast.Ident) string {
	obj := pkg.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return "" // see .Defs doc
	}

	isTopLevel := isTopLevelIdent(obj, true)
	if !isTopLevel {
		return ""
	}

	pos := obj.Pos()
	if !pos.IsValid() {
		return "" // builtin or label
	}

	var syn *ast.File = nil
	for _, pkgSyntax := range pkg.Syntax {
		fset := pkg.Fset
		file := fset.File(pos)
		if file == nil {
			continue
		}
		if file == fset.File(pkgSyntax.Pos()) {
			syn = pkgSyntax
			break
		}
	}
	if syn == nil {
		panic("ident position not found in any syntax files:" + ident.Name + " pos=" + fmt.Sprint(pos) + " pkg=" + pkg.PkgPath)
	}

	// scan ImportSpecs
	for _, decl := range syn.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}
		for _, spec := range genDecl.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}
			importName := GetPackageIdentifier(importSpec)
			if importName == "_" {
				continue
			}
			if importName == "." {
				panic("dot import not supported: " + pkg.PkgPath)
			}
			if importName == ident.Name {
				importPath, err := strconv.Unquote(importSpec.Path.Value)
				if err != nil {
					panic(err)
				}
				return importPath
			}
		}
	}

	return ""
}

// 子の操作を含むのでpreで呼ぶ必要あり
func replaceTopLevel(cursor *astutil.Cursor, pkg *packages.Package, pkgUnfoldStrFn func(PkgPath) string) (update bool) {
	node := cursor.Node()

	ident, ok := node.(*ast.Ident)
	if !ok {
		return false
	}

	par := cursor.Parent()

	obj := pkg.TypesInfo.ObjectOf(ident)
	if _, ok := par.(*ast.StarExpr); ok {
		obj = pkg.TypesInfo.Uses[ident] // pointer type
	}
	if _, ok := par.(*ast.Field); ok {
		obj = pkg.TypesInfo.Uses[ident] // struct field, reciver, etc.
	}

	isTopLevel := isTopLevelIdent(obj, false)
	if !isTopLevel {
		return false
	}

	identPkg := pkg.PkgPath
	prefix := pkgUnfoldStrFn(PkgPath(identPkg))
	if prefix == "" {
		panic("imported unfold package not found: " + identPkg)
	}
	newIdentName := fmt.Sprintf("%s%s%s", prefix, Delim, ident.Name)
	newIdent := &ast.Ident{
		Name:    newIdentName,
		NamePos: ident.NamePos,
	}
	cursor.Replace(newIdent)
	return true
}

// doesn't support GOROOT or vendoring
// see:
//   - https://pkg.go.dev/cmd/go/internal/search#IsStandardImportPath
//   - https://github.com/golang/go/blob/7971fcdf537054608b2443a32f0fbb6dd4eba12a/src/cmd/go/internal/load/pkg.go#L406
func isStandardImportPath(path string) bool {
	i := strings.Index(path, "/")
	if i < 0 {
		i = len(path)
	}
	elem := path[:i]
	return !strings.Contains(elem, ".")
}
