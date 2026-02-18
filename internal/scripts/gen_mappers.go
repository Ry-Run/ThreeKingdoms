package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	defaultRepoRoot    = "."
	defaultInternalDir = "internal"

	defaultBlueprintRelDir = "entity/domain"
	defaultEntityOutRelDir = "entity"

	defaultModelRelDir  = "infra/persistence/model"
	defaultMapperRelDir = "infra/persistence/mapper"

	defaultEntityTag = "entity"
	defaultModelTag  = "model"

	defaultEntityPkg = "entity"
	defaultMapperPkg = "mapper"

	defaultFieldGenFile  = "field_gen.go"
	defaultMapperGenFile = "mapper_gen.go"
)

var (
	flagRoot     = flag.String("root", defaultRepoRoot, "repo root path (must contain go.mod)")
	flagInternal = flag.String("internal", defaultInternalDir, "internal dir under repo root")
	flagModule   = flag.String("module", "", "module dir name under internal (e.g. player). empty = scan all")

	flagBlueprintDir = flag.String("blueprint_dir", defaultBlueprintRelDir, "blueprint dir relative to module dir (contains // entity)")
	flagEntityOutDir = flag.String("entity_out_dir", defaultEntityOutRelDir, "generated entity output dir relative to module dir")
	flagModelDir     = flag.String("model_dir", defaultModelRelDir, "model dir relative to module dir (contains // model)")
	flagMapperDir    = flag.String("mapper_dir", defaultMapperRelDir, "mapper output dir relative to module dir")

	flagEntityTag = flag.String("entity_tag", defaultEntityTag, "comment tag used to mark blueprint entities")
	flagModelTag  = flag.String("model_tag", defaultModelTag, "comment tag used to mark db models")

	flagEntityPkgDefault = flag.String("entity_pkg_default", defaultEntityPkg, "default entity package name if entity output dir is empty")
	flagMapperPkgDefault = flag.String("mapper_pkg_default", defaultMapperPkg, "default mapper package name if mapper dir is empty")

	flagFieldFile  = flag.String("field_file", defaultFieldGenFile, "generated field filename in entity output dir")
	flagMapperFile = flag.String("mapper_file", defaultMapperGenFile, "generated mapper filename in mapper dir")
)

type pkgInfo struct {
	Dir     string
	PkgName string
	ImpPath string
}

type structInfo struct {
	Name          string
	Fields        []fieldInfo
	ModelFieldSet map[string]bool
}

type fieldKind int

const (
	kindOther fieldKind = iota
	kindSlice
	kindMap
)

type fieldInfo struct {
	RawName    string
	ExportName string
	TypeExpr   string
	Kind       fieldKind

	ModelNameOverride string
	ResolvedModelName string
}

func main() {
	flag.Parse()

	rootAbs, err := filepath.Abs(*flagRoot)
	must(err)

	modPath, err := readGoModModulePath(filepath.Join(rootAbs, "go.mod"))
	must(err)

	internalAbs := filepath.Join(rootAbs, *flagInternal)

	modules, err := listModules(internalAbs, *flagModule, *flagBlueprintDir, *flagModelDir)
	must(err)
	if len(modules) == 0 {
		return
	}

	for _, mod := range modules {
		modAbs := filepath.Join(internalAbs, mod)

		blueprintAbs := filepath.Join(modAbs, filepath.FromSlash(*flagBlueprintDir))
		entityOutAbs := filepath.Join(modAbs, filepath.FromSlash(*flagEntityOutDir))
		modelAbs := filepath.Join(modAbs, filepath.FromSlash(*flagModelDir))
		mapperAbs := filepath.Join(modAbs, filepath.FromSlash(*flagMapperDir))

		entityPkg := loadPkgInfoAllowEmpty(rootAbs, modPath, entityOutAbs, *flagEntityPkgDefault)
		modelPkg := loadPkgInfo(rootAbs, modPath, modelAbs)
		mapperPkg := loadPkgInfoAllowEmpty(rootAbs, modPath, mapperAbs, *flagMapperPkgDefault)

		entities := parseTaggedStructs(blueprintAbs, *flagEntityTag, false)
		models := parseTaggedStructs(modelAbs, *flagModelTag, true)

		modelMap := map[string]structInfo{}
		for _, m := range models {
			modelMap[m.Name] = m
		}

		var pairs []structInfo
		for _, e := range entities {
			if m, ok := modelMap[e.Name]; ok {
				pairs = append(pairs, resolveModelNames(e, m))
			}
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].Name < pairs[j].Name })
		if len(pairs) == 0 {
			continue
		}

		must(os.MkdirAll(entityOutAbs, 0o755))
		must(os.MkdirAll(mapperAbs, 0o755))

		must(genFieldCommon(entityPkg, filepath.Join(entityOutAbs, *flagFieldFile)))

		for _, e := range pairs {
			filename := filepath.Join(entityOutAbs, toSnakeLower(e.Name)+"_entity_gen.go")
			must(genOneEntity(entityPkg, e, filename))
		}

		must(genMappers(mapperPkg, modelPkg, entityPkg, pairs, filepath.Join(mapperAbs, *flagMapperFile)))
	}
}

// ---------- model name resolution ----------

func resolveModelNames(entity structInfo, model structInfo) structInfo {
	set := model.ModelFieldSet
	if set == nil {
		set = map[string]bool{}
	}
	for i := range entity.Fields {
		f := &entity.Fields[i]

		if f.ModelNameOverride != "" {
			f.ResolvedModelName = f.ModelNameOverride
			continue
		}
		if set[f.ExportName] {
			f.ResolvedModelName = f.ExportName
			continue
		}
		cand := toInitialismCaps(f.ExportName)
		if set[cand] {
			f.ResolvedModelName = cand
			continue
		}
		cand2 := upperFirst(f.RawName)
		if set[cand2] {
			f.ResolvedModelName = cand2
			continue
		}
		f.ResolvedModelName = f.ExportName
	}
	return entity
}

func toInitialismCaps(exportName string) string {
	if exportName == "Id" {
		return "ID"
	}
	if exportName == "Uid" {
		return "UID"
	}
	if exportName == "Rid" {
		return "RID"
	}
	if strings.HasSuffix(exportName, "Id") {
		return strings.TrimSuffix(exportName, "Id") + "ID"
	}
	return exportName
}

// ---------- module discovery ----------

func listModules(internalAbs, onlyModule, blueprintRel, modelRel string) ([]string, error) {
	if onlyModule != "" {
		modAbs := filepath.Join(internalAbs, onlyModule)
		if !existsDir(modAbs) {
			return nil, nil
		}
		if !existsDir(filepath.Join(modAbs, filepath.FromSlash(blueprintRel))) {
			return nil, nil
		}
		if !existsDir(filepath.Join(modAbs, filepath.FromSlash(modelRel))) {
			return nil, nil
		}
		return []string{onlyModule}, nil
	}

	ents, err := os.ReadDir(internalAbs)
	if err != nil {
		return nil, err
	}

	var mods []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		modAbs := filepath.Join(internalAbs, name)
		if existsDir(filepath.Join(modAbs, filepath.FromSlash(blueprintRel))) &&
			existsDir(filepath.Join(modAbs, filepath.FromSlash(modelRel))) {
			mods = append(mods, name)
		}
	}
	sort.Strings(mods)
	return mods, nil
}

// ---------- parse package / structs ----------

func loadPkgInfo(repoRootAbs, modulePath, dirAbs string) pkgInfo {
	pkgName := mustParsePackageName(dirAbs)
	imp := modulePath + "/" + filepath.ToSlash(mustRel(repoRootAbs, dirAbs))
	return pkgInfo{Dir: dirAbs, PkgName: pkgName, ImpPath: imp}
}

func loadPkgInfoAllowEmpty(repoRootAbs, modulePath, dirAbs, defaultPkg string) pkgInfo {
	pkgName, err := tryParsePackageName(dirAbs)
	if err != nil {
		pkgName = defaultPkg
	}
	imp := modulePath + "/" + filepath.ToSlash(mustRel(repoRootAbs, dirAbs))
	return pkgInfo{Dir: dirAbs, PkgName: pkgName, ImpPath: imp}
}

func mustParsePackageName(dirAbs string) string {
	s, err := tryParsePackageName(dirAbs)
	if err != nil {
		panic(err)
	}
	return s
}

func tryParsePackageName(dirAbs string) (string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dirAbs, func(fi os.FileInfo) bool {
		name := fi.Name()
		if strings.HasSuffix(name, "_test.go") {
			return false
		}
		if strings.HasSuffix(name, "_gen.go") || name == *flagFieldFile || name == *flagMapperFile {
			return false
		}
		return strings.HasSuffix(name, ".go")
	}, parser.PackageClauseOnly)
	if err != nil {
		return "", err
	}
	for name := range pkgs {
		return name, nil
	}
	return "", fmt.Errorf("no go files in %s", dirAbs)
}

var reModelOverride = regexp.MustCompile(`\bmodel\s*[:=]\s*([A-Za-z_][A-Za-z0-9_]*)\b`)

func parseTaggedStructs(dirAbs, tag string, collectModelFieldSet bool) []structInfo {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dirAbs, func(fi os.FileInfo) bool {
		name := fi.Name()
		if strings.HasSuffix(name, "_test.go") {
			return false
		}
		if strings.HasSuffix(name, "_gen.go") || name == *flagFieldFile || name == *flagMapperFile {
			return false
		}
		return strings.HasSuffix(name, ".go")
	}, parser.ParseComments)
	must(err)

	var pkg *ast.Package
	for _, p := range pkgs {
		pkg = p
		break
	}
	if pkg == nil {
		return nil
	}

	var out []structInfo
	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if !hasTag(ts.Doc, tag) && !hasTag(gd.Doc, tag) {
					continue
				}

				si := structInfo{Name: ts.Name.Name}
				if collectModelFieldSet {
					si.ModelFieldSet = map[string]bool{}
				}

				for _, fld := range st.Fields.List {
					if len(fld.Names) == 0 {
						continue
					}
					n := fld.Names[0].Name
					typ := exprString(fset, fld.Type)

					k := kindOther
					switch fld.Type.(type) {
					case *ast.ArrayType:
						k = kindSlice
					case *ast.MapType:
						k = kindMap
					}

					override := ""
					txt := fieldCommentText(fld)
					if m := reModelOverride.FindStringSubmatch(txt); len(m) == 2 {
						override = m[1]
					}

					fi := fieldInfo{
						RawName:           n,
						ExportName:        upperFirst(n),
						TypeExpr:          typ,
						Kind:              k,
						ModelNameOverride: override,
					}
					si.Fields = append(si.Fields, fi)

					if collectModelFieldSet {
						si.ModelFieldSet[n] = true // model 字段名就是 declared name
					}
				}

				out = append(out, si)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func fieldCommentText(f *ast.Field) string {
	var parts []string
	if f.Doc != nil {
		for _, c := range f.Doc.List {
			parts = append(parts, c.Text)
		}
	}
	if f.Comment != nil {
		for _, c := range f.Comment.List {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, " ")
}

func hasTag(cg *ast.CommentGroup, tag string) bool {
	if cg == nil {
		return false
	}
	t := strings.ToLower(tag)
	for _, c := range cg.List {
		if strings.Contains(strings.ToLower(c.Text), t) {
			return true
		}
	}
	return false
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, e)
	return b.String()
}

// ---------- generation: common field ----------

func genFieldCommon(entityPkg pkgInfo, outPath string) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen_persistence_mappers; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", entityPkg.PkgName))
	buf.WriteString("type Field string\n")
	return writeGoFile(outPath, buf.Bytes())
}

// ---------- generation: one entity per file ----------

func genOneEntity(entityPkg pkgInfo, e structInfo, outPath string) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen_persistence_mappers; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", entityPkg.PkgName))

	needTime := false
	for _, f := range e.Fields {
		if f.TypeExpr == "time.Time" || strings.Contains(f.TypeExpr, "time.") {
			needTime = true
		}
	}
	buf.WriteString("import (\n")
	buf.WriteString("\t\"sort\"\n")
	if needTime {
		buf.WriteString("\t\"time\"\n")
	}
	buf.WriteString(")\n\n")

	entityName := e.Name + "entity"
	snapName := e.Name + "EntitySnapshot"
	fieldPrefix := "Field" + e.Name + "_" // ✅ 关键：实体前缀

	// Field constants (value is raw field name)
	buf.WriteString("const (\n")
	for _, f := range e.Fields {
		buf.WriteString(fmt.Sprintf("\t%s%s Field = %q\n", fieldPrefix, f.RawName, f.RawName))
	}
	buf.WriteString(")\n\n")

	// trace
	buf.WriteString(fmt.Sprintf("type %sTrace struct {\n", entityName))
	buf.WriteString("\tdirty bool\n")
	buf.WriteString("\ttrace map[Field]bool\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) mark(f Field) {\n", entityName))
	buf.WriteString("\tt.dirty = true\n")
	buf.WriteString("\tif t.trace == nil {\n\t\tt.trace = make(map[Field]bool, 8)\n\t}\n")
	buf.WriteString("\tt.trace[f] = true\n")
	buf.WriteString("}\n\n")

	// snapshot struct
	buf.WriteString(fmt.Sprintf("type %s struct {\n", snapName))
	for _, f := range e.Fields {
		buf.WriteString(fmt.Sprintf("\t%s %s\n", f.ExportName, f.TypeExpr))
	}
	buf.WriteString("}\n\n")

	// entity struct
	buf.WriteString(fmt.Sprintf("type %s struct {\n", entityName))
	for _, f := range e.Fields {
		buf.WriteString(fmt.Sprintf("\t%s %s\n", f.RawName, f.TypeExpr))
	}
	buf.WriteString(fmt.Sprintf("\t_dt %sTrace\n", entityName))
	buf.WriteString("}\n\n")

	// helper funcs for slice/map
	for _, f := range e.Fields {
		switch f.Kind {
		case kindMap:
			buf.WriteString(fmt.Sprintf("func copyMap_%s(in %s) %s {\n", f.RawName, f.TypeExpr, f.TypeExpr))
			buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
			buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", f.TypeExpr))
			buf.WriteString("\tfor k, v := range in {\n\t\tout[k] = v\n\t}\n")
			buf.WriteString("\treturn out\n")
			buf.WriteString("}\n\n")

			buf.WriteString(fmt.Sprintf("func mapsEqual_%s(a, b %s) bool {\n", f.RawName, f.TypeExpr))
			buf.WriteString("\tif a == nil && b == nil {\n\t\treturn true\n\t}\n")
			buf.WriteString("\tif (a == nil) != (b == nil) {\n\t\treturn false\n\t}\n")
			buf.WriteString("\tif len(a) != len(b) {\n\t\treturn false\n\t}\n")
			buf.WriteString("\tfor k, va := range a {\n\t\tvb, ok := b[k]\n\t\tif !ok || vb != va {\n\t\t\treturn false\n\t\t}\n\t}\n")
			buf.WriteString("\treturn true\n")
			buf.WriteString("}\n\n")

		case kindSlice:
			buf.WriteString(fmt.Sprintf("func slicesEqual_%s(a, b %s) bool {\n", f.RawName, f.TypeExpr))
			buf.WriteString("\tif a == nil && b == nil {\n\t\treturn true\n\t}\n")
			buf.WriteString("\tif (a == nil) != (b == nil) {\n\t\treturn false\n\t}\n")
			buf.WriteString("\tif len(a) != len(b) {\n\t\treturn false\n\t}\n")
			buf.WriteString("\tfor i := range a {\n\t\tif a[i] != b[i] {\n\t\t\treturn false\n\t\t}\n\t}\n")
			buf.WriteString("\treturn true\n")
			buf.WriteString("}\n\n")
		}
	}

	// Hydrate (no dirty)
	buf.WriteString(fmt.Sprintf("func Hydrate%s(s %s) *%s {\n", entityName, snapName, entityName))
	buf.WriteString(fmt.Sprintf("\treturn &%s{\n", entityName))
	for _, f := range e.Fields {
		switch f.Kind {
		case kindSlice:
			buf.WriteString(fmt.Sprintf("\t\t%s: append(%s(nil), s.%s...),\n", f.RawName, f.TypeExpr, f.ExportName))
		case kindMap:
			buf.WriteString(fmt.Sprintf("\t\t%s: copyMap_%s(s.%s),\n", f.RawName, f.RawName, f.ExportName))
		default:
			buf.WriteString(fmt.Sprintf("\t\t%s: s.%s,\n", f.RawName, f.ExportName))
		}
	}
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	// Dirty API
	buf.WriteString(fmt.Sprintf("func (e *%s) Dirty() bool {\n", entityName))
	buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
	buf.WriteString("\treturn e._dt.dirty\n")
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (e *%s) ClearDirty() {\n", entityName))
	buf.WriteString("\tif e == nil {\n\t\treturn\n\t}\n")
	buf.WriteString(fmt.Sprintf("\te._dt = %sTrace{}\n", entityName))
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (e *%s) DirtyFields() []Field {\n", entityName))
	buf.WriteString("\tif e == nil || e._dt.trace == nil {\n\t\treturn nil\n\t}\n")
	buf.WriteString("\tout := make([]Field, 0, len(e._dt.trace))\n")
	buf.WriteString("\tfor k := range e._dt.trace {\n\t\tout = append(out, k)\n\t}\n")
	buf.WriteString("\tsort.Slice(out, func(i, j int) bool { return out[i] < out[j] })\n")
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")

	// Snapshot
	buf.WriteString(fmt.Sprintf("func (e *%s) Snapshot() %s {\n", entityName, snapName))
	buf.WriteString(fmt.Sprintf("\tvar s %s\n", snapName))
	buf.WriteString("\tif e == nil {\n\t\treturn s\n\t}\n")
	for _, f := range e.Fields {
		switch f.Kind {
		case kindSlice:
			buf.WriteString(fmt.Sprintf("\ts.%s = append(%s(nil), e.%s...)\n", f.ExportName, f.TypeExpr, f.RawName))
		case kindMap:
			buf.WriteString(fmt.Sprintf("\ts.%s = copyMap_%s(e.%s)\n", f.ExportName, f.RawName, f.RawName))
		default:
			buf.WriteString(fmt.Sprintf("\ts.%s = e.%s\n", f.ExportName, f.RawName))
		}
	}
	buf.WriteString("\treturn s\n")
	buf.WriteString("}\n\n")

	// Getter + Setter
	for _, f := range e.Fields {
		buf.WriteString(fmt.Sprintf("func (e *%s) %s() %s {\n", entityName, f.ExportName, f.TypeExpr))
		buf.WriteString("\tif e == nil {\n")
		buf.WriteString(fmt.Sprintf("\t\tvar z %s\n", f.TypeExpr))
		buf.WriteString("\t\treturn z\n\t}\n")
		switch f.Kind {
		case kindSlice:
			buf.WriteString(fmt.Sprintf("\treturn append(%s(nil), e.%s...)\n", f.TypeExpr, f.RawName))
		case kindMap:
			buf.WriteString(fmt.Sprintf("\treturn copyMap_%s(e.%s)\n", f.RawName, f.RawName))
		default:
			buf.WriteString(fmt.Sprintf("\treturn e.%s\n", f.RawName))
		}
		buf.WriteString("}\n\n")

		buf.WriteString(fmt.Sprintf("func (e *%s) Set%s(v %s) bool {\n", entityName, f.ExportName, f.TypeExpr))
		buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
		switch f.Kind {
		case kindSlice:
			buf.WriteString(fmt.Sprintf("\tif slicesEqual_%s(e.%s, v) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName))
			buf.WriteString(fmt.Sprintf("\te.%s = append(%s(nil), v...)\n", f.RawName, f.TypeExpr))
			buf.WriteString(fmt.Sprintf("\te._dt.mark(%s%s)\n", fieldPrefix, f.RawName))
			buf.WriteString("\treturn true\n")
		case kindMap:
			buf.WriteString(fmt.Sprintf("\tif mapsEqual_%s(e.%s, v) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName))
			buf.WriteString(fmt.Sprintf("\te.%s = copyMap_%s(v)\n", f.RawName, f.RawName))
			buf.WriteString(fmt.Sprintf("\te._dt.mark(%s%s)\n", fieldPrefix, f.RawName))
			buf.WriteString("\treturn true\n")
		default:
			if f.TypeExpr == "time.Time" {
				buf.WriteString(fmt.Sprintf("\tif e.%s.Equal(v) {\n\t\treturn false\n\t}\n", f.RawName))
			} else {
				buf.WriteString(fmt.Sprintf("\tif e.%s == v {\n\t\treturn false\n\t}\n", f.RawName))
			}
			buf.WriteString(fmt.Sprintf("\te.%s = v\n", f.RawName))
			buf.WriteString(fmt.Sprintf("\te._dt.mark(%s%s)\n", fieldPrefix, f.RawName))
			buf.WriteString("\treturn true\n")
		}
		buf.WriteString("}\n\n")
	}

	return writeGoFile(outPath, buf.Bytes())
}

// ---------- generation: mapper ----------

func genMappers(mapperPkg, modelPkg, entityPkg pkgInfo, entities []structInfo, outPath string) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen_persistence_mappers; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", mapperPkg.PkgName))

	buf.WriteString("import (\n")
	buf.WriteString(fmt.Sprintf("\tmodel %q\n", modelPkg.ImpPath))
	buf.WriteString(fmt.Sprintf("\tentity %q\n", entityPkg.ImpPath))
	buf.WriteString(")\n\n")

	for _, e := range entities {
		entityName := e.Name + "entity"
		snapName := e.Name + "EntitySnapshot"

		buf.WriteString(fmt.Sprintf("func %sModelToEntity(m *model.%s) *entity.%s {\n", e.Name, e.Name, entityName))
		buf.WriteString("\tif m == nil {\n\t\treturn nil\n\t}\n")
		buf.WriteString(fmt.Sprintf("\tvar s entity.%s\n", snapName))
		for _, f := range e.Fields {
			buf.WriteString(fmt.Sprintf("\ts.%s = m.%s\n", f.ExportName, f.ResolvedModelName))
		}
		buf.WriteString(fmt.Sprintf("\treturn entity.Hydrate%s(s)\n", entityName))
		buf.WriteString("}\n\n")

		buf.WriteString(fmt.Sprintf("func %sEntityToModel(e *entity.%s) *model.%s {\n", e.Name, entityName, e.Name))
		buf.WriteString("\tif e == nil {\n\t\treturn nil\n\t}\n")
		buf.WriteString("\ts := e.Snapshot()\n")
		buf.WriteString(fmt.Sprintf("\tout := &model.%s{}\n", e.Name))
		for _, f := range e.Fields {
			buf.WriteString(fmt.Sprintf("\tout.%s = s.%s\n", f.ResolvedModelName, f.ExportName))
		}
		buf.WriteString("\treturn out\n")
		buf.WriteString("}\n\n")
	}

	return writeGoFile(outPath, buf.Bytes())
}

// ---------- helpers ----------

func writeGoFile(path string, src []byte) error {
	formatted, err := format.Source(src)
	if err != nil {
		_ = os.WriteFile(path+".bad", src, 0o644)
		return err
	}
	return os.WriteFile(path, formatted, 0o644)
}

func readGoModModulePath(goModPath string) (string, error) {
	b, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(ln, "module ")), nil
		}
	}
	return "", fmt.Errorf("cannot find module path in %s", goModPath)
}

func existsDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func mustRel(rootAbs, dirAbs string) string {
	rel, err := filepath.Rel(rootAbs, dirAbs)
	if err != nil {
		panic(err)
	}
	return rel
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func toSnakeLower(s string) string {
	var out []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		out = append(out, rune(strings.ToLower(string(r))[0]))
	}
	return string(out)
}
