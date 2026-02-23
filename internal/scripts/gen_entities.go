// gen_entities 用法说明：
// 0. 参与生成的前置条件
//   - domain 结构体必须带 `// entity` 注释（例如 `entity/domain/world.go`）。
//
// 1. 生成产物
//   - XxxEntity：可变实体（含脏标记追踪）。
//   - XxxState：实体字段快照（深拷贝 map/slice）。
//   - XxxEntitySnap：持久化快照封装，包含 Version + State(XxxState)。
//   - XxxDoc：持久化模型（默认输出到 `infra/persistence/model`）。
//   - XxxStateToDoc / XxxDocToState：状态与持久化模型转换方法。
//
// 2. 字段注释能力
//   - `// mapper:ignore` 或 `// mapper=ignore`：字段不进入生成产物。
//
// 3. domain 方法镜像
//   - 对 blueprint(domain) 中“无参无返回”的方法，生成同名 Save 代理方法。
//   - 代理方法会把 Save 映射到 domain 结构体，调用 domain 方法后再回填 Save。
//
// 4. 集合快捷方法
//   - map 字段额外生成：
//   - Get<Field> / Len<Field> / ForEach<Field>
//   - Replace<Field>(map)
//   - Put<Field>(key, value) / Put<Field>Many(entries)
//   - Del<Field>(key) / Del<Field>Many(keys) / Clear<Field>()
//   - ForEach<Field>(fn)（只读迭代，避免频繁拷贝）
//   - slice 字段额外生成：
//   - Len<Field> / At<Field> / ForEach<Field>
//   - Replace<Field>(slice)
//   - Append<Field>(values...)
//   - Set<Field>At(index, value)
//   - Remove<Field>At(index) / SwapRemove<Field>At(index)
//   - DirtyChanges()：返回集合字段增量（map 的 set/del，slice 的 append/set/remove）。
//
// 5. 常用命令
//   - 仅生成 world 模块：
//     `go run internal/scripts/gen_entities.go -root . -module world`
//   - 仅生成 player 模块：
//     `go run internal/scripts/gen_entities.go -root . -module player`
//   - 关闭 model 生成（仅生成 entity）：
//     `go run internal/scripts/gen_entities.go -root . -module world -gen_model=false`
//   - 自定义 model 输出目录：
//     `go run internal/scripts/gen_entities.go -root . -module world -model_out_dir infra/persistence/model`
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
	"strconv"
	"strings"
)

const (
	defaultRepoRoot    = "."
	defaultInternalDir = "internal"

	defaultBlueprintRelDir = "entity/domain"
	defaultEntityOutRelDir = "entity"
	defaultModelOutRelDir  = "infra/persistence/model"

	defaultEntityTag = "entity"

	defaultEntityPkg = "entity"
	defaultModelPkg  = "model"

	defaultFieldGenFile = "field_gen.go"
)

var (
	flagRoot     = flag.String("root", defaultRepoRoot, "repo root path (must contain go.mod)")
	flagInternal = flag.String("internal", defaultInternalDir, "internal dir under repo root")
	flagModule   = flag.String("module", "", "module dir name under internal (e.g. player). empty = scan all")

	flagBlueprintDir = flag.String("blueprint_dir", defaultBlueprintRelDir, "blueprint dir relative to module dir (contains // entity)")
	flagEntityOutDir = flag.String("entity_out_dir", defaultEntityOutRelDir, "generated entity output dir relative to module dir")
	flagModelOutDir  = flag.String("model_out_dir", defaultModelOutRelDir, "generated model output dir relative to module dir")
	flagGenModel     = flag.Bool("gen_model", true, "whether to generate model docs and codecs")

	flagEntityTag = flag.String("entity_tag", defaultEntityTag, "comment tag used to mark blueprint entities")

	flagEntityPkgDefault = flag.String("entity_pkg_default", defaultEntityPkg, "default entity package name if entity output dir is empty")
	flagModelPkgDefault  = flag.String("model_pkg_default", defaultModelPkg, "default model package name if model output dir is empty")

	flagFieldFile = flag.String("field_file", defaultFieldGenFile, "generated field filename in entity output dir")
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
	Methods       []methodInfo
}

type importRef struct {
	Alias string
	Path  string
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
	Ignore            bool
	DecodeMethod      string
	EncodeMethod      string
}

type methodInfo struct {
	Name            string
	NoArgNoReturn   bool
	ReceiverTypeRaw string
}

func main() {
	flag.Parse()

	rootAbs, err := filepath.Abs(*flagRoot)
	must(err)

	modPath, err := readGoModModulePath(filepath.Join(rootAbs, "go.mod"))
	must(err)

	internalAbs := filepath.Join(rootAbs, *flagInternal)

	modules, err := listModules(internalAbs, *flagModule, *flagBlueprintDir)
	must(err)
	if len(modules) == 0 {
		return
	}

	for _, mod := range modules {
		modAbs := filepath.Join(internalAbs, mod)

		blueprintAbs := filepath.Join(modAbs, filepath.FromSlash(*flagBlueprintDir))
		entityOutAbs := filepath.Join(modAbs, filepath.FromSlash(*flagEntityOutDir))
		modelOutAbs := filepath.Join(modAbs, filepath.FromSlash(*flagModelOutDir))

		entityPkg := loadPkgInfoAllowEmpty(rootAbs, modPath, entityOutAbs, *flagEntityPkgDefault)
		blueprintPkg := loadPkgInfo(rootAbs, modPath, blueprintAbs)
		modelPkg := loadPkgInfoAllowEmpty(rootAbs, modPath, modelOutAbs, *flagModelPkgDefault)

		entities := parseTaggedStructs(blueprintAbs, *flagEntityTag, false, true)
		sort.Slice(entities, func(i, j int) bool { return entities[i].Name < entities[j].Name })
		if len(entities) == 0 {
			continue
		}
		entityNames := collectEntityNames(entities)

		must(os.MkdirAll(entityOutAbs, 0o755))

		must(genFieldCommon(entityPkg, filepath.Join(entityOutAbs, *flagFieldFile)))
		must(genBasicTypesAndEnums(entityPkg, blueprintAbs, entityNames, filepath.Join(entityOutAbs, "types_gen.go"), nil))

		for _, e := range entities {
			filename := filepath.Join(entityOutAbs, toSnakeLower(e.Name)+"_entity_gen.go")
			must(genOneEntity(entityPkg, blueprintPkg, e, entityNames, filename))
		}

		if *flagGenModel {
			must(os.MkdirAll(modelOutAbs, 0o755))
			must(genBasicTypesAndEnums(modelPkg, blueprintAbs, entityNames, filepath.Join(modelOutAbs, "types_gen.go"), &entityPkg))
			for _, e := range entities {
				if !shouldGenerateModelDoc(e) {
					continue
				}
				filename := filepath.Join(modelOutAbs, toSnakeLower(e.Name)+"_doc_gen.go")
				must(genOneModel(modelPkg, entityPkg, e, entityNames, filename))
			}
		}
	}
}

func shouldGenerateModelDoc(e structInfo) bool {
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if strings.Contains(f.TypeExpr, "Entity") {
			return false
		}
	}
	return true
}

// ---------- model name resolution ----------

func resolveModelNames(entity structInfo, model structInfo) structInfo {
	set := model.ModelFieldSet
	if set == nil {
		set = map[string]bool{}
	}
	for i := range entity.Fields {
		f := &entity.Fields[i]
		if f.Ignore {
			continue
		}

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

func listModules(internalAbs, onlyModule, blueprintRel string) ([]string, error) {
	if onlyModule != "" {
		modAbs := filepath.Join(internalAbs, onlyModule)
		if !existsDir(modAbs) {
			return nil, nil
		}
		if !existsDir(filepath.Join(modAbs, filepath.FromSlash(blueprintRel))) {
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
		if existsDir(filepath.Join(modAbs, filepath.FromSlash(blueprintRel))) {
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
		if strings.HasSuffix(name, "_gen.go") || name == *flagFieldFile {
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

var (
	reModelOverride = regexp.MustCompile(`\bmodel\s*[:=]\s*([A-Za-z_][A-Za-z0-9_]*)\b`)
	reMapperIgnore  = regexp.MustCompile(`\bmapper\s*[:=]\s*ignore\b`)
	reDecodeMethod  = regexp.MustCompile(`\bdecode\s*[:=]\s*"?([A-Za-z_][A-Za-z0-9_]*)"?`)
	reEncodeMethod  = regexp.MustCompile(`\bencode\s*[:=]\s*"?([A-Za-z_][A-Za-z0-9_]*)"?`)
)

func parseTaggedStructs(dirAbs, tag string, collectModelFieldSet bool, collectMethods bool) []structInfo {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dirAbs, func(fi os.FileInfo) bool {
		name := fi.Name()
		if strings.HasSuffix(name, "_test.go") {
			return false
		}
		if strings.HasSuffix(name, "_gen.go") || name == *flagFieldFile {
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
	indexByName := map[string]int{}
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
					decodeMethod := firstSubmatch(reDecodeMethod, txt)
					encodeMethod := firstSubmatch(reEncodeMethod, txt)

					fi := fieldInfo{
						RawName:           n,
						ExportName:        upperFirst(n),
						TypeExpr:          typ,
						Kind:              k,
						ModelNameOverride: override,
						Ignore:            reMapperIgnore.MatchString(txt),
						DecodeMethod:      decodeMethod,
						EncodeMethod:      encodeMethod,
					}
					si.Fields = append(si.Fields, fi)

					if collectModelFieldSet {
						si.ModelFieldSet[n] = true // model 字段名就是 declared name
					}
				}

				out = append(out, si)
				indexByName[si.Name] = len(out) - 1
			}
		}
	}

	if collectMethods && len(indexByName) > 0 {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
					continue
				}
				recvName, recvTypeRaw, ok := receiverTypeName(fd.Recv.List[0].Type)
				if !ok {
					continue
				}
				idx, ok := indexByName[recvName]
				if !ok {
					continue
				}
				out[idx].Methods = append(out[idx].Methods, methodInfo{
					Name:            fd.Name.Name,
					NoArgNoReturn:   fieldListArity(fd.Type.Params) == 0 && fieldListArity(fd.Type.Results) == 0,
					ReceiverTypeRaw: recvTypeRaw,
				})
			}
		}
		for i := range out {
			sort.Slice(out[i].Methods, func(a, b int) bool { return out[i].Methods[a].Name < out[i].Methods[b].Name })
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

func firstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

func fieldListArity(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	n := 0
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			n++
			continue
		}
		n += len(f.Names)
	}
	return n
}

func receiverTypeName(recv ast.Expr) (name string, raw string, ok bool) {
	switch t := recv.(type) {
	case *ast.Ident:
		return t.Name, t.Name, true
	case *ast.StarExpr:
		id, ok := t.X.(*ast.Ident)
		if !ok {
			return "", "", false
		}
		return id.Name, "*" + id.Name, true
	default:
		return "", "", false
	}
}

func collectNoArgNoReturnMethods(methods []methodInfo) []methodInfo {
	seen := map[string]bool{}
	out := make([]methodInfo, 0, len(methods))
	for _, m := range methods {
		if !m.NoArgNoReturn {
			continue
		}
		if seen[m.Name] {
			continue
		}
		seen[m.Name] = true
		out = append(out, m)
	}
	return out
}

func canGenerateSnapshotMethodProxy(e structInfo) bool {
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if f.RawName != f.ExportName {
			return false
		}
	}
	return true
}

func collectHookMethods(fields []fieldInfo, decode bool) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f.Ignore {
			continue
		}
		methodName := f.EncodeMethod
		if decode {
			methodName = f.DecodeMethod
		}
		if methodName == "" || seen[methodName] {
			continue
		}
		seen[methodName] = true
		out = append(out, methodName)
	}
	return out
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
	buf.WriteString("// Code generated by gen_entities; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", entityPkg.PkgName))
	buf.WriteString("type Field string\n")
	return writeGoFile(outPath, buf.Bytes())
}

func genBasicTypesAndEnums(entityPkg pkgInfo, blueprintAbs string, entityNames map[string]bool, outPath string, aliasTo *pkgInfo) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, blueprintAbs, func(fi os.FileInfo) bool {
		name := fi.Name()
		if strings.HasSuffix(name, "_test.go") {
			return false
		}
		if strings.HasSuffix(name, "_gen.go") || name == *flagFieldFile {
			return false
		}
		return strings.HasSuffix(name, ".go")
	}, parser.ParseComments)
	if err != nil {
		return err
	}

	var pkg *ast.Package
	for _, p := range pkgs {
		pkg = p
		break
	}
	if pkg == nil {
		_ = os.Remove(outPath)
		return nil
	}

	fileNames := make([]string, 0, len(pkg.Files))
	for name := range pkg.Files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	typeNames := make(map[string]bool, 8)
	typeOrder := make([]string, 0, 8)
	typeDecls := make(map[string]string, 8)
	constDecls := make([]string, 0, 4)
	imports := make(map[string]string, 4)
	importOrder := make([]importRef, 0, 4)

	for _, fileName := range fileNames {
		f := pkg.Files[fileName]
		fileImports := fileImportAliasMap(f)
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
				if entityNames[ts.Name.Name] {
					continue
				}
				// 非 entity 标记的 struct（如 Building/Army/General）也需要导出到生成包，
				// 否则其它生成文件在引用 []Building / *Army 等类型时会找不到定义。
				typeNames[ts.Name.Name] = true
				typeOrder = append(typeOrder, ts.Name.Name)
				typeDecls[ts.Name.Name] = renderTypeSpecDecl(fset, ts)
				if aliasTo == nil {
					addNodeImports(imports, &importOrder, fileImports, ts.Type)
				}
			}
		}
	}

	if len(typeOrder) == 0 {
		_ = os.Remove(outPath)
		return nil
	}

	if aliasTo != nil {
		if _, exists := imports["entity"]; !exists {
			imports["entity"] = aliasTo.ImpPath
			importOrder = append([]importRef{{Alias: "entity", Path: aliasTo.ImpPath}}, importOrder...)
		}
	}

	for _, fileName := range fileNames {
		f := pkg.Files[fileName]
		fileImports := fileImportAliasMap(f)
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			if !constBlockUsesTypes(gd, typeNames) {
				continue
			}
			constDecls = append(constDecls, renderNode(fset, gd))
			addNodeImports(imports, &importOrder, fileImports, gd)
		}
	}

	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen_entities; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", entityPkg.PkgName))
	if len(importOrder) > 0 {
		buf.WriteString("import (\n")
		for _, im := range importOrder {
			if im.Alias == "" {
				buf.WriteString(fmt.Sprintf("\t%q\n", im.Path))
				continue
			}
			buf.WriteString(fmt.Sprintf("\t%s %q\n", im.Alias, im.Path))
		}
		buf.WriteString(")\n\n")
	}
	for _, typeName := range typeOrder {
		if aliasTo != nil {
			buf.WriteString(fmt.Sprintf("type %s = entity.%s", typeName, typeName))
		} else {
			buf.WriteString(typeDecls[typeName])
		}
		buf.WriteString("\n\n")
	}
	for _, decl := range constDecls {
		buf.WriteString(decl)
		buf.WriteString("\n\n")
	}
	return writeGoFile(outPath, buf.Bytes())
}

func renderTypeSpecDecl(fset *token.FileSet, ts *ast.TypeSpec) string {
	return "type " + renderNode(fset, ts)
}

func renderNode(fset *token.FileSet, node ast.Node) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, node)
	return b.String()
}

func constBlockUsesTypes(gd *ast.GenDecl, typeNames map[string]bool) bool {
	currentType := ""
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		if vs.Type != nil {
			if id, ok := vs.Type.(*ast.Ident); ok {
				currentType = id.Name
			} else {
				currentType = ""
			}
		}
		if currentType != "" && typeNames[currentType] {
			return true
		}
	}
	return false
}

func fileImportAliasMap(f *ast.File) map[string]string {
	if f == nil || len(f.Imports) == 0 {
		return nil
	}
	out := make(map[string]string, len(f.Imports))
	for _, imp := range f.Imports {
		if imp == nil || imp.Path == nil {
			continue
		}
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path == "" {
			continue
		}
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			alias = filepath.Base(path)
		}
		if alias == "_" || alias == "." || alias == "" {
			continue
		}
		out[alias] = path
	}
	return out
}

func addNodeImports(imports map[string]string, order *[]importRef, aliasMap map[string]string, node ast.Node) {
	if node == nil || len(aliasMap) == 0 {
		return
	}
	aliases := collectSelectorAliases(node)
	sort.Strings(aliases)
	for _, alias := range aliases {
		path, ok := aliasMap[alias]
		if !ok {
			continue
		}
		if old, exists := imports[alias]; exists {
			if old == path {
				continue
			}
			// 同名不同包极少见；保留首次出现避免生成冲突 import。
			continue
		}
		imports[alias] = path
		*order = append(*order, importRef{Alias: alias, Path: path})
	}
}

func collectSelectorAliases(node ast.Node) []string {
	seen := make(map[string]bool, 4)
	out := make([]string, 0, 4)
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Name == "" {
			return true
		}
		if seen[id.Name] {
			return true
		}
		seen[id.Name] = true
		out = append(out, id.Name)
		return true
	})
	return out
}

// ---------- generation: one entity per file ----------

func genOneEntity(entityPkg, blueprintPkg pkgInfo, e structInfo, entityNames map[string]bool, outPath string) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen_entities; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", entityPkg.PkgName))

	type nestedMapField struct {
		field        fieldInfo
		keyType      string
		nestedEntity string
	}

	needTime := false
	hasMap := false
	needReflect := false
	nestedMapFields := make([]nestedMapField, 0, 2)
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if f.TypeExpr == "time.Time" || strings.Contains(f.TypeExpr, "time.") {
			needTime = true
		}
		if f.Kind == kindMap {
			hasMap = true
		}
		if f.Kind == kindSlice {
			if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
				if _, nested := directNestedEntity(elemType, entityNames); nested {
					needReflect = true
				}
			}
		}
		if f.Kind == kindMap && containsNestedEntityInMapValue(f.TypeExpr, entityNames) {
			needReflect = true
		}
		if keyType, nestedEntity, ok := mapNestedEntity(f.TypeExpr, entityNames); ok {
			nestedMapFields = append(nestedMapFields, nestedMapField{
				field:        f,
				keyType:      keyType,
				nestedEntity: nestedEntity,
			})
		}
	}
	mirrorMethods := collectNoArgNoReturnMethods(e.Methods)
	canMirrorMethods := canGenerateSnapshotMethodProxy(e)

	buf.WriteString("import (\n")
	if hasMap {
		buf.WriteString("\t\"fmt\"\n")
	}
	buf.WriteString("\t\"sort\"\n")
	if needTime {
		buf.WriteString("\t\"time\"\n")
	}
	if needReflect {
		buf.WriteString("\t\"reflect\"\n")
	}
	if canMirrorMethods && len(mirrorMethods) > 0 {
		buf.WriteString(fmt.Sprintf("\tdomain %q\n", blueprintPkg.ImpPath))
	}
	buf.WriteString(")\n\n")

	entityName := e.Name + "Entity"
	stateName := e.Name + "State"
	entitySnapName := e.Name + "EntitySnap"
	fieldPrefix := "Field" + e.Name + "_" // ✅ 关键：实体前缀

	// Field constants (value is raw field name)
	buf.WriteString("const (\n")
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		buf.WriteString(fmt.Sprintf("\t%s%s Field = %q\n", fieldPrefix, f.RawName, f.RawName))
	}
	buf.WriteString(")\n\n")

	// trace
	buf.WriteString(fmt.Sprintf("type %sCollectionChange struct {\n", entityName))
	buf.WriteString("\tFullReplace       bool\n")
	buf.WriteString("\tMapSet            map[string]any\n")
	buf.WriteString("\tMapDeleteKeys     []string\n")
	buf.WriteString("\tSliceSet          map[int]any\n")
	buf.WriteString("\tSliceAppend       []any\n")
	buf.WriteString("\tSliceRemoveAt     []int\n")
	buf.WriteString("\tSliceSwapRemoveAt []int\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("type %sCollectionChangeInner struct {\n", entityName))
	buf.WriteString("\tfullReplace       bool\n")
	buf.WriteString("\tmapSet            map[string]any\n")
	buf.WriteString("\tmapDelete         map[string]struct{}\n")
	buf.WriteString("\tsliceSet          map[int]any\n")
	buf.WriteString("\tsliceAppend       []any\n")
	buf.WriteString("\tsliceRemoveAt     []int\n")
	buf.WriteString("\tsliceSwapRemoveAt []int\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("type %sTrace struct {\n", entityName))
	buf.WriteString("\tdirty bool\n")
	buf.WriteString("\ttrace map[Field]bool\n")
	buf.WriteString(fmt.Sprintf("\tchanges map[Field]*%sCollectionChangeInner\n", entityName))
	for _, nf := range nestedMapFields {
		buf.WriteString(fmt.Sprintf("\tchildDirty_%s map[%s]struct{}\n", nf.field.RawName, nf.keyType))
	}
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) mark(f Field) {\n", entityName))
	buf.WriteString("\tt.dirty = true\n")
	buf.WriteString("\tif t.trace == nil {\n\t\tt.trace = make(map[Field]bool, 8)\n\t}\n")
	buf.WriteString("\tt.trace[f] = true\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) ensureChange(f Field) *%sCollectionChangeInner {\n", entityName, entityName))
	buf.WriteString("\tif t.changes == nil {\n")
	buf.WriteString(fmt.Sprintf("\t\tt.changes = make(map[Field]*%sCollectionChangeInner, 4)\n", entityName))
	buf.WriteString("\t}\n")
	buf.WriteString("\tch, ok := t.changes[f]\n")
	buf.WriteString("\tif !ok || ch == nil {\n")
	buf.WriteString(fmt.Sprintf("\t\tch = &%sCollectionChangeInner{}\n", entityName))
	buf.WriteString("\t\tt.changes[f] = ch\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn ch\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) markFullReplace(f Field) {\n", entityName))
	buf.WriteString("\tt.mark(f)\n")
	buf.WriteString("\tch := t.ensureChange(f)\n")
	buf.WriteString("\tch.fullReplace = true\n")
	buf.WriteString("\tch.mapSet = nil\n")
	buf.WriteString("\tch.mapDelete = nil\n")
	buf.WriteString("\tch.sliceSet = nil\n")
	buf.WriteString("\tch.sliceAppend = nil\n")
	buf.WriteString("\tch.sliceRemoveAt = nil\n")
	buf.WriteString("\tch.sliceSwapRemoveAt = nil\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) markMapSet(f Field, key string, value any) {\n", entityName))
	buf.WriteString("\tt.mark(f)\n")
	buf.WriteString("\tch := t.ensureChange(f)\n")
	buf.WriteString("\tif ch.fullReplace {\n\t\treturn\n\t}\n")
	buf.WriteString("\tif ch.mapSet == nil {\n\t\tch.mapSet = make(map[string]any, 4)\n\t}\n")
	buf.WriteString("\tch.mapSet[key] = value\n")
	buf.WriteString("\tif ch.mapDelete != nil {\n\t\tdelete(ch.mapDelete, key)\n\t}\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) markMapDelete(f Field, key string) {\n", entityName))
	buf.WriteString("\tt.mark(f)\n")
	buf.WriteString("\tch := t.ensureChange(f)\n")
	buf.WriteString("\tif ch.fullReplace {\n\t\treturn\n\t}\n")
	buf.WriteString("\tif ch.mapDelete == nil {\n\t\tch.mapDelete = make(map[string]struct{}, 4)\n\t}\n")
	buf.WriteString("\tch.mapDelete[key] = struct{}{}\n")
	buf.WriteString("\tif ch.mapSet != nil {\n\t\tdelete(ch.mapSet, key)\n\t}\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) markSliceAppend(f Field, values ...any) {\n", entityName))
	buf.WriteString("\tif len(values) == 0 {\n\t\treturn\n\t}\n")
	buf.WriteString("\tt.mark(f)\n")
	buf.WriteString("\tch := t.ensureChange(f)\n")
	buf.WriteString("\tif ch.fullReplace {\n\t\treturn\n\t}\n")
	buf.WriteString("\tch.sliceAppend = append(ch.sliceAppend, values...)\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) markSliceSet(f Field, index int, value any) {\n", entityName))
	buf.WriteString("\tt.mark(f)\n")
	buf.WriteString("\tch := t.ensureChange(f)\n")
	buf.WriteString("\tif ch.fullReplace {\n\t\treturn\n\t}\n")
	buf.WriteString("\tif ch.sliceSet == nil {\n\t\tch.sliceSet = make(map[int]any, 4)\n\t}\n")
	buf.WriteString("\tch.sliceSet[index] = value\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) markSliceRemoveAt(f Field, index int) {\n", entityName))
	buf.WriteString("\tt.mark(f)\n")
	buf.WriteString("\tch := t.ensureChange(f)\n")
	buf.WriteString("\tif ch.fullReplace {\n\t\treturn\n\t}\n")
	buf.WriteString("\tch.sliceRemoveAt = append(ch.sliceRemoveAt, index)\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (t *%sTrace) markSliceSwapRemoveAt(f Field, index int) {\n", entityName))
	buf.WriteString("\tt.mark(f)\n")
	buf.WriteString("\tch := t.ensureChange(f)\n")
	buf.WriteString("\tif ch.fullReplace {\n\t\treturn\n\t}\n")
	buf.WriteString("\tch.sliceSwapRemoveAt = append(ch.sliceSwapRemoveAt, index)\n")
	buf.WriteString("}\n\n")
	for _, nf := range nestedMapFields {
		buf.WriteString(fmt.Sprintf("func (t *%sTrace) markChildDirty_%s(f Field, key %s) {\n", entityName, nf.field.RawName, nf.keyType))
		buf.WriteString("\tt.mark(f)\n")
		buf.WriteString(fmt.Sprintf("\tif t.childDirty_%s == nil {\n", nf.field.RawName))
		buf.WriteString(fmt.Sprintf("\t\tt.childDirty_%s = make(map[%s]struct{}, 8)\n", nf.field.RawName, nf.keyType))
		buf.WriteString("\t}\n")
		buf.WriteString(fmt.Sprintf("\tt.childDirty_%s[key] = struct{}{}\n", nf.field.RawName))
		buf.WriteString("}\n\n")

		buf.WriteString(fmt.Sprintf("func (t *%sTrace) clearChildDirty_%s(key %s) {\n", entityName, nf.field.RawName, nf.keyType))
		buf.WriteString(fmt.Sprintf("\tif t.childDirty_%s == nil {\n\t\treturn\n\t}\n", nf.field.RawName))
		buf.WriteString(fmt.Sprintf("\tdelete(t.childDirty_%s, key)\n", nf.field.RawName))
		buf.WriteString("}\n\n")

		buf.WriteString(fmt.Sprintf("func (t *%sTrace) childDirtyKeys_%s() []%s {\n", entityName, nf.field.RawName, nf.keyType))
		buf.WriteString(fmt.Sprintf("\tif len(t.childDirty_%s) == 0 {\n\t\treturn nil\n\t}\n", nf.field.RawName))
		buf.WriteString(fmt.Sprintf("\tout := make([]%s, 0, len(t.childDirty_%s))\n", nf.keyType, nf.field.RawName))
		buf.WriteString(fmt.Sprintf("\tfor key := range t.childDirty_%s {\n\t\tout = append(out, key)\n\t}\n", nf.field.RawName))
		buf.WriteString("\tsort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]) < fmt.Sprint(out[j]) })\n")
		buf.WriteString("\treturn out\n")
		buf.WriteString("}\n\n")
	}

	// state struct
	buf.WriteString(fmt.Sprintf("type %s struct {\n", stateName))
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		buf.WriteString(fmt.Sprintf("\t%s %s\n", f.ExportName, stateTypeExpr(f.TypeExpr, entityNames)))
	}
	buf.WriteString("}\n\n")

	// snap envelope: version + state
	buf.WriteString(fmt.Sprintf("type %s struct {\n", entitySnapName))
	buf.WriteString("\tVersion    uint64\n")
	buf.WriteString(fmt.Sprintf("\tState      %s\n", stateName))
	buf.WriteString("\tDirtyFields []Field\n")
	buf.WriteString(fmt.Sprintf("\tChanges    map[Field]%sCollectionChange\n", entityName))
	for _, nf := range nestedMapFields {
		buf.WriteString(fmt.Sprintf("\t%sDirtyKeys []%s\n", nf.field.ExportName, nf.keyType))
	}
	buf.WriteString("}\n\n")

	// entity struct
	buf.WriteString(fmt.Sprintf("type %s struct {\n", entityName))
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		buf.WriteString(fmt.Sprintf("\t%s %s\n", f.RawName, runtimeTypeExpr(f.TypeExpr, entityNames)))
	}
	buf.WriteString(fmt.Sprintf("\t_dt %sTrace\n", entityName))
	buf.WriteString("}\n\n")

	// helper funcs for slice/map
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		switch f.Kind {
		case kindMap:
			if keyType, nestedEntity, ok := mapNestedEntity(f.TypeExpr, entityNames); ok {
				stateMapType := fmt.Sprintf("map[%s]%sState", keyType, nestedEntity)
				runtimeMapType := fmt.Sprintf("map[%s]*%sEntity", keyType, nestedEntity)
				stateValueType := nestedEntity + "State"

				buf.WriteString(fmt.Sprintf("func copyMap_%s(in %s) %s {\n", f.RawName, stateMapType, stateMapType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateMapType))
				buf.WriteString("\tfor k, v := range in {\n\t\tout[k] = v\n\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func mapsEqual_%s(a, b %s) bool {\n", f.RawName, stateMapType))
				buf.WriteString("\tif a == nil && b == nil {\n\t\treturn true\n\t}\n")
				buf.WriteString("\treturn false\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func hydrateMap_%s(in %s) %s {\n", f.RawName, stateMapType, runtimeMapType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", runtimeMapType))
				buf.WriteString("\tfor k, v := range in {\n")
				buf.WriteString(fmt.Sprintf("\t\tout[k] = Hydrate%sEntity(v)\n", nestedEntity))
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func snapshotMap_%s(in %s) %s {\n", f.RawName, runtimeMapType, stateMapType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateMapType))
				buf.WriteString("\tfor k, v := range in {\n")
				buf.WriteString("\t\tif v == nil {\n")
				buf.WriteString(fmt.Sprintf("\t\t\tvar z %s\n", stateValueType))
				buf.WriteString("\t\t\tout[k] = z\n")
				buf.WriteString("\t\t\tcontinue\n")
				buf.WriteString("\t\t}\n")
				buf.WriteString("\t\tout[k] = v.Save()\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")
			} else if containsNestedEntityInMapValue(f.TypeExpr, entityNames) {
				stateMapType := stateTypeExpr(f.TypeExpr, entityNames)
				runtimeMapType := runtimeTypeExpr(f.TypeExpr, entityNames)
				_, valueType, _ := parseMapType(f.TypeExpr)
				stateValueType := stateTypeExpr(valueType, entityNames)

				if code, ok := genRecursiveConvertFunc("copyMapValue_"+f.RawName, valueType, entityNames, recursiveConvCloneState, "", "", ""); ok {
					buf.WriteString(code)
				}
				if code, ok := genRecursiveConvertFunc("hydrateMapValue_"+f.RawName, valueType, entityNames, recursiveConvStateToRuntime, "", "", ""); ok {
					buf.WriteString(code)
				}
				if code, ok := genRecursiveConvertFunc("snapshotMapValue_"+f.RawName, valueType, entityNames, recursiveConvRuntimeToState, "", "", ""); ok {
					buf.WriteString(code)
				}

				buf.WriteString(fmt.Sprintf("func copyMap_%s(in %s) %s {\n", f.RawName, stateMapType, stateMapType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateMapType))
				buf.WriteString("\tfor k, v := range in {\n")
				buf.WriteString(fmt.Sprintf("\t\tout[k] = copyMapValue_%s(v)\n", f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func mapsEqual_%s(a, b %s) bool {\n", f.RawName, stateMapType))
				buf.WriteString("\treturn reflect.DeepEqual(a, b)\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func hydrateMap_%s(in %s) %s {\n", f.RawName, stateMapType, runtimeMapType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", runtimeMapType))
				buf.WriteString("\tfor k, v := range in {\n")
				buf.WriteString(fmt.Sprintf("\t\tout[k] = hydrateMapValue_%s(v)\n", f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func snapshotMap_%s(in %s) %s {\n", f.RawName, runtimeMapType, stateMapType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateMapType))
				buf.WriteString("\tfor k, v := range in {\n")
				buf.WriteString(fmt.Sprintf("\t\tvar sv %s\n", stateValueType))
				buf.WriteString(fmt.Sprintf("\t\tsv = snapshotMapValue_%s(v)\n", f.RawName))
				buf.WriteString("\t\tout[k] = sv\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")
			} else {
				stateMapType := stateTypeExpr(f.TypeExpr, entityNames)
				buf.WriteString(fmt.Sprintf("func copyMap_%s(in %s) %s {\n", f.RawName, stateMapType, stateMapType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateMapType))
				buf.WriteString("\tfor k, v := range in {\n\t\tout[k] = v\n\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func mapsEqual_%s(a, b %s) bool {\n", f.RawName, stateMapType))
				buf.WriteString("\tif a == nil && b == nil {\n\t\treturn true\n\t}\n")
				buf.WriteString("\tif (a == nil) != (b == nil) {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tif len(a) != len(b) {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tfor k, va := range a {\n\t\tvb, ok := b[k]\n\t\tif !ok || vb != va {\n\t\t\treturn false\n\t\t}\n\t}\n")
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			}

		case kindSlice:
			if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
				if nestedEntity, nested := directNestedEntity(elemType, entityNames); nested {
					stateSliceType := "[]" + nestedEntity + "State"
					runtimeSliceType := "[]*" + nestedEntity + "Entity"
					stateElemType := nestedEntity + "State"

					buf.WriteString(fmt.Sprintf("func hydrateSlice_%s(in %s) %s {\n", f.RawName, stateSliceType, runtimeSliceType))
					buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
					buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", runtimeSliceType))
					buf.WriteString("\tfor i, v := range in {\n")
					buf.WriteString(fmt.Sprintf("\t\tout[i] = Hydrate%sEntity(v)\n", nestedEntity))
					buf.WriteString("\t}\n")
					buf.WriteString("\treturn out\n")
					buf.WriteString("}\n\n")

					buf.WriteString(fmt.Sprintf("func snapshotSlice_%s(in %s) %s {\n", f.RawName, runtimeSliceType, stateSliceType))
					buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
					buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateSliceType))
					buf.WriteString("\tfor i, v := range in {\n")
					buf.WriteString("\t\tif v == nil {\n")
					buf.WriteString(fmt.Sprintf("\t\t\tvar z %s\n", stateElemType))
					buf.WriteString("\t\t\tout[i] = z\n")
					buf.WriteString("\t\t\tcontinue\n")
					buf.WriteString("\t\t}\n")
					buf.WriteString("\t\tout[i] = v.Save()\n")
					buf.WriteString("\t}\n")
					buf.WriteString("\treturn out\n")
					buf.WriteString("}\n\n")

					buf.WriteString(fmt.Sprintf("func slicesEqual_%s(a, b %s) bool {\n", f.RawName, stateSliceType))
					buf.WriteString("\tif a == nil && b == nil {\n\t\treturn true\n\t}\n")
					buf.WriteString("\tif (a == nil) != (b == nil) {\n\t\treturn false\n\t}\n")
					buf.WriteString("\tif len(a) != len(b) {\n\t\treturn false\n\t}\n")
					buf.WriteString("\tfor i := range a {\n")
					buf.WriteString("\t\tif !reflect.DeepEqual(a[i], b[i]) {\n\t\t\treturn false\n\t\t}\n")
					buf.WriteString("\t}\n")
					buf.WriteString("\treturn true\n")
					buf.WriteString("}\n\n")
					break
				}
			}
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
	buf.WriteString(fmt.Sprintf("func Hydrate%s(s %s) *%s {\n", entityName, stateName, entityName))
	buf.WriteString(fmt.Sprintf("\treturn &%s{\n", entityName))
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		switch f.Kind {
		case kindSlice:
			if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
				if _, nested := directNestedEntity(elemType, entityNames); nested {
					buf.WriteString(fmt.Sprintf("\t\t%s: hydrateSlice_%s(s.%s),\n", f.RawName, f.RawName, f.ExportName))
					break
				}
			}
			buf.WriteString(fmt.Sprintf("\t\t%s: append(%s(nil), s.%s...),\n", f.RawName, stateTypeExpr(f.TypeExpr, entityNames), f.ExportName))
		case kindMap:
			if _, _, ok := mapNestedEntity(f.TypeExpr, entityNames); ok {
				buf.WriteString(fmt.Sprintf("\t\t%s: hydrateMap_%s(s.%s),\n", f.RawName, f.RawName, f.ExportName))
			} else if containsNestedEntityInMapValue(f.TypeExpr, entityNames) {
				buf.WriteString(fmt.Sprintf("\t\t%s: hydrateMap_%s(s.%s),\n", f.RawName, f.RawName, f.ExportName))
			} else {
				buf.WriteString(fmt.Sprintf("\t\t%s: copyMap_%s(s.%s),\n", f.RawName, f.RawName, f.ExportName))
			}
		default:
			if nestedEntity, ok := directNestedEntity(f.TypeExpr, entityNames); ok {
				buf.WriteString(fmt.Sprintf("\t\t%s: Hydrate%sEntity(s.%s),\n", f.RawName, nestedEntity, f.ExportName))
			} else {
				buf.WriteString(fmt.Sprintf("\t\t%s: s.%s,\n", f.RawName, f.ExportName))
			}
		}
	}
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	// Dirty API
	buf.WriteString(fmt.Sprintf("func (e *%s) Dirty() bool {\n", entityName))
	buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
	buf.WriteString("\tif e._dt.dirty {\n\t\treturn true\n\t}\n")
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if f.Kind != kindOther {
			continue
		}
		if _, ok := directNestedEntity(f.TypeExpr, entityNames); !ok {
			continue
		}
		buf.WriteString(fmt.Sprintf("\tif e.%s != nil && e.%s.Dirty() {\n\t\treturn true\n\t}\n", f.RawName, f.RawName))
	}
	buf.WriteString("\treturn false\n")
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (e *%s) ClearDirty() {\n", entityName))
	buf.WriteString("\tif e == nil {\n\t\treturn\n\t}\n")
	buf.WriteString(fmt.Sprintf("\te._dt = %sTrace{}\n", entityName))
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if f.Kind != kindOther {
			continue
		}
		if _, ok := directNestedEntity(f.TypeExpr, entityNames); !ok {
			continue
		}
		buf.WriteString(fmt.Sprintf("\tif e.%s != nil {\n\t\te.%s.ClearDirty()\n\t}\n", f.RawName, f.RawName))
	}
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (e *%s) DirtyFields() []Field {\n", entityName))
	buf.WriteString("\tif e == nil {\n\t\treturn nil\n\t}\n")
	buf.WriteString("\ttrace := make(map[Field]bool, len(e._dt.trace)+4)\n")
	buf.WriteString("\tfor k := range e._dt.trace {\n\t\ttrace[k] = true\n\t}\n")
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if f.Kind != kindOther {
			continue
		}
		if _, ok := directNestedEntity(f.TypeExpr, entityNames); !ok {
			continue
		}
		buf.WriteString(fmt.Sprintf("\tif e.%s != nil && e.%s.Dirty() {\n\t\ttrace[%s%s] = true\n\t}\n", f.RawName, f.RawName, fieldPrefix, f.RawName))
	}
	buf.WriteString("\tif len(trace) == 0 {\n\t\treturn nil\n\t}\n")
	buf.WriteString("\tout := make([]Field, 0, len(trace))\n")
	buf.WriteString("\tfor k := range trace {\n\t\tout = append(out, k)\n\t}\n")
	buf.WriteString("\tsort.Slice(out, func(i, j int) bool { return out[i] < out[j] })\n")
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func (e *%s) DirtyChanges() map[Field]%sCollectionChange {\n", entityName, entityName))
	buf.WriteString("\tif e == nil || len(e._dt.changes) == 0 {\n\t\treturn nil\n\t}\n")
	buf.WriteString(fmt.Sprintf("\tout := make(map[Field]%sCollectionChange, len(e._dt.changes))\n", entityName))
	buf.WriteString("\tfor f, ch := range e._dt.changes {\n")
	buf.WriteString("\t\tif ch == nil {\n\t\t\tcontinue\n\t\t}\n")
	buf.WriteString(fmt.Sprintf("\t\titem := %sCollectionChange{\n", entityName))
	buf.WriteString("\t\t\tFullReplace: ch.fullReplace,\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif len(ch.mapSet) > 0 {\n")
	buf.WriteString("\t\t\titem.MapSet = make(map[string]any, len(ch.mapSet))\n")
	buf.WriteString("\t\t\tfor k, v := range ch.mapSet {\n\t\t\t\titem.MapSet[k] = v\n\t\t\t}\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif len(ch.mapDelete) > 0 {\n")
	buf.WriteString("\t\t\tkeys := make([]string, 0, len(ch.mapDelete))\n")
	buf.WriteString("\t\t\tfor k := range ch.mapDelete {\n\t\t\t\tkeys = append(keys, k)\n\t\t\t}\n")
	buf.WriteString("\t\t\tsort.Strings(keys)\n")
	buf.WriteString("\t\t\titem.MapDeleteKeys = keys\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif len(ch.sliceSet) > 0 {\n")
	buf.WriteString("\t\t\titem.SliceSet = make(map[int]any, len(ch.sliceSet))\n")
	buf.WriteString("\t\t\tfor idx, v := range ch.sliceSet {\n\t\t\t\titem.SliceSet[idx] = v\n\t\t\t}\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif len(ch.sliceAppend) > 0 {\n")
	buf.WriteString("\t\t\titem.SliceAppend = append([]any(nil), ch.sliceAppend...)\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif len(ch.sliceRemoveAt) > 0 {\n")
	buf.WriteString("\t\t\titem.SliceRemoveAt = append([]int(nil), ch.sliceRemoveAt...)\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif len(ch.sliceSwapRemoveAt) > 0 {\n")
	buf.WriteString("\t\t\titem.SliceSwapRemoveAt = append([]int(nil), ch.sliceSwapRemoveAt...)\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tout[f] = item\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")
	buf.WriteString(fmt.Sprintf("func clone%sCollectionChange(in %sCollectionChange) %sCollectionChange {\n", entityName, entityName, entityName))
	buf.WriteString("\tout := in\n")
	buf.WriteString("\tif in.MapSet != nil {\n")
	buf.WriteString("\t\tout.MapSet = make(map[string]any, len(in.MapSet))\n")
	buf.WriteString("\t\tfor k, v := range in.MapSet {\n\t\t\tout.MapSet[k] = v\n\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tif in.MapDeleteKeys != nil {\n")
	buf.WriteString("\t\tout.MapDeleteKeys = append([]string(nil), in.MapDeleteKeys...)\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tif in.SliceSet != nil {\n")
	buf.WriteString("\t\tout.SliceSet = make(map[int]any, len(in.SliceSet))\n")
	buf.WriteString("\t\tfor idx, v := range in.SliceSet {\n\t\t\tout.SliceSet[idx] = v\n\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tif in.SliceAppend != nil {\n")
	buf.WriteString("\t\tout.SliceAppend = append([]any(nil), in.SliceAppend...)\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tif in.SliceRemoveAt != nil {\n")
	buf.WriteString("\t\tout.SliceRemoveAt = append([]int(nil), in.SliceRemoveAt...)\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tif in.SliceSwapRemoveAt != nil {\n")
	buf.WriteString("\t\tout.SliceSwapRemoveAt = append([]int(nil), in.SliceSwapRemoveAt...)\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")

	// Save
	buf.WriteString(fmt.Sprintf("func (e *%s) Save() %s {\n", entityName, stateName))
	buf.WriteString(fmt.Sprintf("\tvar s %s\n", stateName))
	buf.WriteString("\tif e == nil {\n\t\treturn s\n\t}\n")
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		switch f.Kind {
		case kindSlice:
			if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
				if _, nested := directNestedEntity(elemType, entityNames); nested {
					buf.WriteString(fmt.Sprintf("\ts.%s = snapshotSlice_%s(e.%s)\n", f.ExportName, f.RawName, f.RawName))
					break
				}
			}
			buf.WriteString(fmt.Sprintf("\ts.%s = append(%s(nil), e.%s...)\n", f.ExportName, stateTypeExpr(f.TypeExpr, entityNames), f.RawName))
		case kindMap:
			if _, _, ok := mapNestedEntity(f.TypeExpr, entityNames); ok {
				buf.WriteString(fmt.Sprintf("\ts.%s = snapshotMap_%s(e.%s)\n", f.ExportName, f.RawName, f.RawName))
			} else if containsNestedEntityInMapValue(f.TypeExpr, entityNames) {
				buf.WriteString(fmt.Sprintf("\ts.%s = snapshotMap_%s(e.%s)\n", f.ExportName, f.RawName, f.RawName))
			} else {
				buf.WriteString(fmt.Sprintf("\ts.%s = copyMap_%s(e.%s)\n", f.ExportName, f.RawName, f.RawName))
			}
		default:
			if nestedEntity, ok := directNestedEntity(f.TypeExpr, entityNames); ok {
				buf.WriteString(fmt.Sprintf("\tif e.%s != nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\ts.%s = e.%s.Save()\n", f.ExportName, f.RawName))
				buf.WriteString("\t} else {\n")
				buf.WriteString(fmt.Sprintf("\t\tvar z %sState\n", nestedEntity))
				buf.WriteString(fmt.Sprintf("\t\ts.%s = z\n", f.ExportName))
				buf.WriteString("\t}\n")
			} else {
				buf.WriteString(fmt.Sprintf("\ts.%s = e.%s\n", f.ExportName, f.RawName))
			}
		}
	}
	buf.WriteString("\treturn s\n")
	buf.WriteString("}\n\n")

	// EntitySnap helpers
	buf.WriteString(fmt.Sprintf("func New%s(version uint64, e *%s) *%s {\n", entitySnapName, entityName, entitySnapName))
	buf.WriteString("\tif e == nil {\n\t\treturn nil\n\t}\n")
	buf.WriteString("\tdirtyFields := e.DirtyFields()\n")
	buf.WriteString("\tchanges := e.DirtyChanges()\n")
	buf.WriteString(fmt.Sprintf("\treturn &%s{\n", entitySnapName))
	buf.WriteString("\t\tVersion:     version,\n")
	buf.WriteString("\t\tState:       e.Save(),\n")
	buf.WriteString("\t\tDirtyFields: dirtyFields,\n")
	buf.WriteString("\t\tChanges:     changes,\n")
	for _, nf := range nestedMapFields {
		buf.WriteString(fmt.Sprintf("\t\t%sDirtyKeys: e._dt.childDirtyKeys_%s(),\n", nf.field.ExportName, nf.field.RawName))
	}
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (s *%s) Clone() *%s {\n", entitySnapName, entitySnapName))
	buf.WriteString("\tif s == nil {\n\t\treturn nil\n\t}\n")
	buf.WriteString(fmt.Sprintf("\tout := &%s{Version: s.Version}\n", entitySnapName))
	buf.WriteString("\tout.State = s.State\n")
	buf.WriteString("\tout.DirtyFields = append([]Field(nil), s.DirtyFields...)\n")
	buf.WriteString("\tif len(s.Changes) > 0 {\n")
	buf.WriteString(fmt.Sprintf("\t\tout.Changes = make(map[Field]%sCollectionChange, len(s.Changes))\n", entityName))
	buf.WriteString("\t\tfor f, ch := range s.Changes {\n")
	buf.WriteString(fmt.Sprintf("\t\t\tout.Changes[f] = clone%sCollectionChange(ch)\n", entityName))
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t}\n")
	for _, nf := range nestedMapFields {
		buf.WriteString(fmt.Sprintf("\tout.%sDirtyKeys = append([]%s(nil), s.%sDirtyKeys...)\n", nf.field.ExportName, nf.keyType, nf.field.ExportName))
	}
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		switch f.Kind {
		case kindSlice:
			buf.WriteString(fmt.Sprintf("\tout.State.%s = append(%s(nil), s.State.%s...)\n", f.ExportName, stateTypeExpr(f.TypeExpr, entityNames), f.ExportName))
		case kindMap:
			buf.WriteString(fmt.Sprintf("\tout.State.%s = copyMap_%s(s.State.%s)\n", f.ExportName, f.RawName, f.ExportName))
		}
	}
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")

	// Save domain method mirrors
	if canMirrorMethods {
		for _, m := range mirrorMethods {
			buf.WriteString(fmt.Sprintf("func (s *%s) %s() {\n", stateName, m.Name))
			buf.WriteString("\tif s == nil {\n\t\treturn\n\t}\n")
			buf.WriteString(fmt.Sprintf("\td := domain.%s{\n", e.Name))
			for _, f := range e.Fields {
				if f.Ignore {
					continue
				}
				buf.WriteString(fmt.Sprintf("\t\t%s: s.%s,\n", f.ExportName, f.ExportName))
			}
			buf.WriteString("\t}\n")
			buf.WriteString(fmt.Sprintf("\td.%s()\n", m.Name))
			for _, f := range e.Fields {
				if f.Ignore {
					continue
				}
				buf.WriteString(fmt.Sprintf("\ts.%s = d.%s\n", f.ExportName, f.ExportName))
			}
			buf.WriteString("}\n\n")
		}
	}

	// Getter + Setter
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}

		switch f.Kind {
		case kindMap:
			keyType, valueType, ok := parseMapType(f.TypeExpr)
			if !ok {
				continue
			}
			if _, nestedEntity, nested := mapNestedEntity(f.TypeExpr, entityNames); nested {
				stateValueType := nestedEntity + "State"
				stateMapType := fmt.Sprintf("map[%s]%s", keyType, stateValueType)
				runtimeMapType := fmt.Sprintf("map[%s]*%sEntity", keyType, nestedEntity)

				buf.WriteString(fmt.Sprintf("func (e *%s) Get%s(key %s) (%s, bool) {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString(fmt.Sprintf("\tvar z %s\n", stateValueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn z, false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tv, ok := e.%s[key]\n", f.RawName))
				buf.WriteString("\tif !ok || v == nil {\n\t\treturn z, false\n\t}\n")
				buf.WriteString("\treturn v.Save(), true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Len%s() int {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn 0\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn len(e.%s)\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) ForEach%s(fn func(key %s, value %s)) {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor k, v := range e.%s {\n", f.RawName))
				buf.WriteString("\t\tif v == nil {\n\t\t\tcontinue\n\t\t}\n")
				buf.WriteString("\t\tfn(k, v.Save())\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Range%s(fn func(key %s, value %s) bool) {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor k, v := range e.%s {\n", f.RawName))
				buf.WriteString("\t\tif v == nil {\n\t\t\tcontinue\n\t\t}\n")
				buf.WriteString("\t\tif !fn(k, v.Save()) {\n\t\t\treturn\n\t\t}\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Dirty%sKeys() []%s {\n", entityName, f.ExportName, keyType))
				buf.WriteString("\tif e == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn e._dt.childDirtyKeys_%s()\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Replace%s(v %s) bool {\n", entityName, f.ExportName, stateMapType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif mapsEqual_%s(snapshotMap_%s(e.%s), v) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = hydrateMap_%s(v)\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Put%s(key %s, value %s) bool {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s = make(%s)\n", f.RawName, runtimeMapType))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\te.%s[key] = Hydrate%sEntity(value)\n", f.RawName, nestedEntity))
				buf.WriteString(fmt.Sprintf("\te._dt.markMapSet(%s%s, fmt.Sprint(key), value)\n", fieldPrefix, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markChildDirty_%s(%s%s, key)\n", f.RawName, fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Put%sMany(entries %s) bool {\n", entityName, f.ExportName, stateMapType))
				buf.WriteString("\tif e == nil || len(entries) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s = make(%s, len(entries))\n", f.RawName, runtimeMapType))
				buf.WriteString("\t}\n")
				buf.WriteString("\tchanged := false\n")
				buf.WriteString("\tfor k, v := range entries {\n")
				buf.WriteString(fmt.Sprintf("\t\te.%s[k] = Hydrate%sEntity(v)\n", f.RawName, nestedEntity))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markMapSet(%s%s, fmt.Sprint(k), v)\n", fieldPrefix, f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markChildDirty_%s(%s%s, k)\n", f.RawName, fieldPrefix, f.RawName))
				buf.WriteString("\t\tchanged = true\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn changed\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Update%s(key %s, fn func(value *%sEntity)) bool {\n", entityName, f.ExportName, keyType, nestedEntity))
				buf.WriteString("\tif e == nil || fn == nil || e." + f.RawName + " == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tv, ok := e.%s[key]\n", f.RawName))
				buf.WriteString("\tif !ok || v == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tfn(v)\n")
				buf.WriteString(fmt.Sprintf("\te._dt.markChildDirty_%s(%s%s, key)\n", f.RawName, fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Del%s(key %s) bool {\n", entityName, f.ExportName, keyType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif _, ok := e.%s[key]; !ok {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tdelete(e.%s, key)\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markMapDelete(%s%s, fmt.Sprint(key))\n", fieldPrefix, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.clearChildDirty_%s(key)\n", f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Del%sMany(keys []%s) bool {\n", entityName, f.ExportName, keyType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || len(keys) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tchanged := false\n")
				buf.WriteString("\tfor _, key := range keys {\n")
				buf.WriteString(fmt.Sprintf("\t\tif _, ok := e.%s[key]; !ok {\n\t\t\tcontinue\n\t\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\tdelete(e.%s, key)\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markMapDelete(%s%s, fmt.Sprint(key))\n", fieldPrefix, f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.clearChildDirty_%s(key)\n", f.RawName))
				buf.WriteString("\t\tchanged = true\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn changed\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Clear%s() bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif len(e.%s) == 0 {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = nil\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.childDirty_%s = nil\n", f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			} else if containsNestedEntityInMapValue(f.TypeExpr, entityNames) {
				stateMapType := stateTypeExpr(f.TypeExpr, entityNames)
				runtimeMapType := runtimeTypeExpr(f.TypeExpr, entityNames)
				stateValueType := stateTypeExpr(valueType, entityNames)

				buf.WriteString(fmt.Sprintf("func (e *%s) Get%s(key %s) (%s, bool) {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString(fmt.Sprintf("\tvar z %s\n", stateValueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn z, false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tv, ok := e.%s[key]\n", f.RawName))
				buf.WriteString("\tif !ok {\n\t\treturn z, false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn snapshotMapValue_%s(v), true\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Len%s() int {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn 0\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn len(e.%s)\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) ForEach%s(fn func(key %s, value %s)) {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor k, v := range e.%s {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\tfn(k, snapshotMapValue_%s(v))\n", f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Range%s(fn func(key %s, value %s) bool) {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor k, v := range e.%s {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\tif !fn(k, snapshotMapValue_%s(v)) {\n\t\t\treturn\n\t\t}\n", f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Replace%s(v %s) bool {\n", entityName, f.ExportName, stateMapType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif mapsEqual_%s(snapshotMap_%s(e.%s), v) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = hydrateMap_%s(v)\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Put%s(key %s, value %s) bool {\n", entityName, f.ExportName, keyType, stateValueType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s = make(%s)\n", f.RawName, runtimeMapType))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\tif old, ok := e.%s[key]; ok && reflect.DeepEqual(snapshotMapValue_%s(old), value) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s[key] = hydrateMapValue_%s(value)\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markMapSet(%s%s, fmt.Sprint(key), copyMapValue_%s(value))\n", fieldPrefix, f.RawName, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Put%sMany(entries %s) bool {\n", entityName, f.ExportName, stateMapType))
				buf.WriteString("\tif e == nil || len(entries) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s = make(%s, len(entries))\n", f.RawName, runtimeMapType))
				buf.WriteString("\t}\n")
				buf.WriteString("\tchanged := false\n")
				buf.WriteString("\tfor k, v := range entries {\n")
				buf.WriteString(fmt.Sprintf("\t\tif old, ok := e.%s[k]; ok && reflect.DeepEqual(snapshotMapValue_%s(old), v) {\n\t\t\tcontinue\n\t\t}\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s[k] = hydrateMapValue_%s(v)\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markMapSet(%s%s, fmt.Sprint(k), copyMapValue_%s(v))\n", fieldPrefix, f.RawName, f.RawName))
				buf.WriteString("\t\tchanged = true\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn changed\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Update%s(key %s, fn func(value %s)) bool {\n", entityName, f.ExportName, keyType, runtimeTypeExpr(valueType, entityNames)))
				buf.WriteString("\tif e == nil || fn == nil || e." + f.RawName + " == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tv, ok := e.%s[key]\n", f.RawName))
				buf.WriteString("\tif !ok {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tbefore := snapshotMapValue_%s(v)\n", f.RawName))
				buf.WriteString("\tfn(v)\n")
				buf.WriteString(fmt.Sprintf("\tafter := snapshotMapValue_%s(v)\n", f.RawName))
				buf.WriteString("\tif reflect.DeepEqual(before, after) {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\te._dt.markMapSet(%s%s, fmt.Sprint(key), copyMapValue_%s(after))\n", fieldPrefix, f.RawName, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Del%s(key %s) bool {\n", entityName, f.ExportName, keyType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif _, ok := e.%s[key]; !ok {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tdelete(e.%s, key)\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markMapDelete(%s%s, fmt.Sprint(key))\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Del%sMany(keys []%s) bool {\n", entityName, f.ExportName, keyType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || len(keys) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tchanged := false\n")
				buf.WriteString("\tfor _, key := range keys {\n")
				buf.WriteString(fmt.Sprintf("\t\tif _, ok := e.%s[key]; !ok {\n\t\t\tcontinue\n\t\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\tdelete(e.%s, key)\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markMapDelete(%s%s, fmt.Sprint(key))\n", fieldPrefix, f.RawName))
				buf.WriteString("\t\tchanged = true\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn changed\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Clear%s() bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif len(e.%s) == 0 {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = nil\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			} else {
				buf.WriteString(fmt.Sprintf("func (e *%s) Get%s(key %s) (%s, bool) {\n", entityName, f.ExportName, keyType, valueType))
				buf.WriteString(fmt.Sprintf("\tvar z %s\n", valueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn z, false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tv, ok := e.%s[key]\n", f.RawName))
				buf.WriteString("\tif !ok {\n\t\treturn z, false\n\t}\n")
				buf.WriteString("\treturn v, true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Len%s() int {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn 0\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn len(e.%s)\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) ForEach%s(fn func(key %s, value %s)) {\n", entityName, f.ExportName, keyType, valueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor k, v := range e.%s {\n", f.RawName))
				buf.WriteString("\t\tfn(k, v)\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Range%s(fn func(key %s, value %s) bool) {\n", entityName, f.ExportName, keyType, valueType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor k, v := range e.%s {\n", f.RawName))
				buf.WriteString("\t\tif !fn(k, v) {\n\t\t\treturn\n\t\t}\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Replace%s(v %s) bool {\n", entityName, f.ExportName, stateTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif mapsEqual_%s(e.%s, v) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = copyMap_%s(v)\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Put%s(key %s, value %s) bool {\n", entityName, f.ExportName, keyType, valueType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s = make(%s)\n", f.RawName, runtimeTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\tif old, ok := e.%s[key]; ok && old == value {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s[key] = value\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markMapSet(%s%s, fmt.Sprint(key), value)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Put%sMany(entries %s) bool {\n", entityName, f.ExportName, stateTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\tif e == nil || len(entries) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s = make(%s, len(entries))\n", f.RawName, runtimeTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\t}\n")
				buf.WriteString("\tchanged := false\n")
				buf.WriteString("\tfor k, v := range entries {\n")
				buf.WriteString(fmt.Sprintf("\t\tif old, ok := e.%s[k]; ok && old == v {\n\t\t\tcontinue\n\t\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s[k] = v\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markMapSet(%s%s, fmt.Sprint(k), v)\n", fieldPrefix, f.RawName))
				buf.WriteString("\t\tchanged = true\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn changed\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Del%s(key %s) bool {\n", entityName, f.ExportName, keyType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif _, ok := e.%s[key]; !ok {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tdelete(e.%s, key)\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markMapDelete(%s%s, fmt.Sprint(key))\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Del%sMany(keys []%s) bool {\n", entityName, f.ExportName, keyType))
				buf.WriteString("\tif e == nil || e." + f.RawName + " == nil || len(keys) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tchanged := false\n")
				buf.WriteString("\tfor _, key := range keys {\n")
				buf.WriteString(fmt.Sprintf("\t\tif _, ok := e.%s[key]; !ok {\n\t\t\tcontinue\n\t\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\tdelete(e.%s, key)\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markMapDelete(%s%s, fmt.Sprint(key))\n", fieldPrefix, f.RawName))
				buf.WriteString("\t\tchanged = true\n")
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn changed\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Clear%s() bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif len(e.%s) == 0 {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = nil\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			}

		case kindSlice:
			elemType, ok := parseSliceElemType(f.TypeExpr)
			if !ok {
				continue
			}
			if nestedEntity, nested := directNestedEntity(elemType, entityNames); nested {
				stateElemType := nestedEntity + "State"
				runtimeElemType := "*" + nestedEntity + "Entity"

				buf.WriteString(fmt.Sprintf("func (e *%s) Len%s() int {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn 0\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn len(e.%s)\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) At%s(index int) (%s, bool) {\n", entityName, f.ExportName, stateElemType))
				buf.WriteString(fmt.Sprintf("\tvar z %s\n", stateElemType))
				buf.WriteString("\tif e == nil {\n\t\treturn z, false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn z, false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tv := e.%s[index]\n", f.RawName))
				buf.WriteString("\tif v == nil {\n\t\treturn z, true\n\t}\n")
				buf.WriteString("\treturn v.Save(), true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) ForEach%s(fn func(index int, value %s)) {\n", entityName, f.ExportName, stateElemType))
				buf.WriteString("\tif e == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor i, v := range e.%s {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\tvar state %s\n", stateElemType))
				buf.WriteString("\t\tif v != nil {\n\t\t\tstate = v.Save()\n\t\t}\n")
				buf.WriteString("\t\tfn(i, state)\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Range%s(fn func(index int, value %s) bool) {\n", entityName, f.ExportName, stateElemType))
				buf.WriteString("\tif e == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor i, v := range e.%s {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\tvar state %s\n", stateElemType))
				buf.WriteString("\t\tif v != nil {\n\t\t\tstate = v.Save()\n\t\t}\n")
				buf.WriteString("\t\tif !fn(i, state) {\n\t\t\treturn\n\t\t}\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Replace%s(v %s) bool {\n", entityName, f.ExportName, stateTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif slicesEqual_%s(snapshotSlice_%s(e.%s), v) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = hydrateSlice_%s(v)\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Append%s(values ...%s) bool {\n", entityName, f.ExportName, stateElemType))
				buf.WriteString("\tif e == nil || len(values) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tfor _, v := range values {\n")
				buf.WriteString(fmt.Sprintf("\t\trv := Hydrate%sEntity(v)\n", nestedEntity))
				buf.WriteString(fmt.Sprintf("\t\te.%s = append(e.%s, rv)\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te._dt.markSliceAppend(%s%s, v)\n", fieldPrefix, f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Set%sAt(index int, value %s) bool {\n", entityName, f.ExportName, stateElemType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tvar oldState %s\n", stateElemType))
				buf.WriteString(fmt.Sprintf("\tif e.%s[index] != nil {\n\t\toldState = e.%s[index].Save()\n\t}\n", f.RawName, f.RawName))
				buf.WriteString("\tif reflect.DeepEqual(oldState, value) {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\te.%s[index] = Hydrate%sEntity(value)\n", f.RawName, nestedEntity))
				buf.WriteString(fmt.Sprintf("\te._dt.markSliceSet(%s%s, index, value)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Update%sAt(index int, fn func(value %s)) bool {\n", entityName, f.ExportName, runtimeElemType))
				buf.WriteString("\tif e == nil || fn == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tv := e.%s[index]\n", f.RawName))
				buf.WriteString("\tif v == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString("\tbefore := v.Save()\n")
				buf.WriteString("\tfn(v)\n")
				buf.WriteString("\tafter := v.Save()\n")
				buf.WriteString("\tif reflect.DeepEqual(before, after) {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\te._dt.markSliceSet(%s%s, index, after)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Remove%sAt(index int) bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = append(e.%s[:index], e.%s[index+1:]...)\n", f.RawName, f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markSliceRemoveAt(%s%s, index)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) SwapRemove%sAt(index int) bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tlast := len(e.%s) - 1\n", f.RawName))
				buf.WriteString("\tif index != last {\n")
				buf.WriteString(fmt.Sprintf("\t\te.%s[index] = e.%s[last]\n", f.RawName, f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\te.%s = e.%s[:last]\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markSliceSwapRemoveAt(%s%s, index)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Clear%s() bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif len(e.%s) == 0 {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = nil\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			} else {
				apiElemType := elemType

				buf.WriteString(fmt.Sprintf("func (e *%s) Len%s() int {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn 0\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn len(e.%s)\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) At%s(index int) (%s, bool) {\n", entityName, f.ExportName, apiElemType))
				buf.WriteString(fmt.Sprintf("\tvar z %s\n", apiElemType))
				buf.WriteString("\tif e == nil {\n\t\treturn z, false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn z, false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\treturn e.%s[index], true\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) ForEach%s(fn func(index int, value %s)) {\n", entityName, f.ExportName, apiElemType))
				buf.WriteString("\tif e == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor i, v := range e.%s {\n", f.RawName))
				buf.WriteString("\t\tfn(i, v)\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Range%s(fn func(index int, value %s) bool) {\n", entityName, f.ExportName, apiElemType))
				buf.WriteString("\tif e == nil || fn == nil {\n\t\treturn\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tfor i, v := range e.%s {\n", f.RawName))
				buf.WriteString("\t\tif !fn(i, v) {\n\t\t\treturn\n\t\t}\n")
				buf.WriteString("\t}\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Replace%s(v %s) bool {\n", entityName, f.ExportName, stateTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif slicesEqual_%s(e.%s, v) {\n\t\treturn false\n\t}\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = append(%s(nil), v...)\n", f.RawName, runtimeTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Append%s(values ...%s) bool {\n", entityName, f.ExportName, apiElemType))
				buf.WriteString("\tif e == nil || len(values) == 0 {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\te.%s = append(e.%s, values...)\n", f.RawName, f.RawName))
				buf.WriteString("\tfor _, v := range values {\n")
				buf.WriteString(fmt.Sprintf("\t\te._dt.markSliceAppend(%s%s, v)\n", fieldPrefix, f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Set%sAt(index int, value %s) bool {\n", entityName, f.ExportName, apiElemType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tif e.%s[index] == value {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s[index] = value\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markSliceSet(%s%s, index, value)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Remove%sAt(index int) bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = append(e.%s[:index], e.%s[index+1:]...)\n", f.RawName, f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markSliceRemoveAt(%s%s, index)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) SwapRemove%sAt(index int) bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif index < 0 || index >= len(e.%s) {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\tlast := len(e.%s) - 1\n", f.RawName))
				buf.WriteString("\tif index != last {\n")
				buf.WriteString(fmt.Sprintf("\t\te.%s[index] = e.%s[last]\n", f.RawName, f.RawName))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\te.%s = e.%s[:last]\n", f.RawName, f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markSliceSwapRemoveAt(%s%s, index)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Clear%s() bool {\n", entityName, f.ExportName))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif len(e.%s) == 0 {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = nil\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.markFullReplace(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			}

		default:
			if nestedEntity, ok := directNestedEntity(f.TypeExpr, entityNames); ok {
				buf.WriteString(fmt.Sprintf("func (e *%s) %s() *%sEntity {\n", entityName, f.ExportName, nestedEntity))
				buf.WriteString("\tif e == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn e.%s\n", f.RawName))
				buf.WriteString("}\n\n")

				stateValueType := nestedEntity + "State"
				buf.WriteString(fmt.Sprintf("func (e *%s) Set%s(v %s) bool {\n", entityName, f.ExportName, stateValueType))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tnext := Hydrate%sEntity(v)\n", nestedEntity))
				buf.WriteString(fmt.Sprintf("\tif e.%s == next {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = next\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.mark(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Set%sEntity(v *%sEntity) bool {\n", entityName, f.ExportName, nestedEntity))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == v {\n\t\treturn false\n\t}\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te.%s = v\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.mark(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Update%s(fn func(value *%sEntity)) bool {\n", entityName, f.ExportName, nestedEntity))
				buf.WriteString("\tif e == nil || fn == nil {\n\t\treturn false\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tif e.%s == nil {\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\t\te.%s = &%sEntity{}\n", f.RawName, nestedEntity))
				buf.WriteString("\t}\n")
				buf.WriteString(fmt.Sprintf("\tfn(e.%s)\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.mark(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			} else {
				buf.WriteString(fmt.Sprintf("func (e *%s) %s() %s {\n", entityName, f.ExportName, runtimeTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\tif e == nil {\n")
				buf.WriteString(fmt.Sprintf("\t\tvar z %s\n", runtimeTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\t\treturn z\n\t}\n")
				buf.WriteString(fmt.Sprintf("\treturn e.%s\n", f.RawName))
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func (e *%s) Set%s(v %s) bool {\n", entityName, f.ExportName, runtimeTypeExpr(f.TypeExpr, entityNames)))
				buf.WriteString("\tif e == nil {\n\t\treturn false\n\t}\n")
				if f.TypeExpr == "time.Time" {
					buf.WriteString(fmt.Sprintf("\tif e.%s.Equal(v) {\n\t\treturn false\n\t}\n", f.RawName))
				} else {
					buf.WriteString(fmt.Sprintf("\tif e.%s == v {\n\t\treturn false\n\t}\n", f.RawName))
				}
				buf.WriteString(fmt.Sprintf("\te.%s = v\n", f.RawName))
				buf.WriteString(fmt.Sprintf("\te._dt.mark(%s%s)\n", fieldPrefix, f.RawName))
				buf.WriteString("\treturn true\n")
				buf.WriteString("}\n\n")
			}
		}
	}

	return writeGoFile(outPath, buf.Bytes())
}

func genOneModel(modelPkg, entityPkg pkgInfo, e structInfo, entityNames map[string]bool, outPath string) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen_entities; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", modelPkg.PkgName))

	needTime := false
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if f.TypeExpr == "time.Time" || strings.Contains(f.TypeExpr, "time.") {
			needTime = true
		}
	}

	buf.WriteString("import (\n")
	buf.WriteString(fmt.Sprintf("\tentity %q\n", entityPkg.ImpPath))
	if needTime {
		buf.WriteString("\t\"time\"\n")
	}
	buf.WriteString(")\n\n")

	stateName := e.Name + "State"
	docName := e.Name + "Doc"

	buf.WriteString(fmt.Sprintf("type %s struct {\n", docName))
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		bufTag := toSnakeCase(f.RawName)
		buf.WriteString(fmt.Sprintf("\t%s %s `bson:%q`\n", f.ExportName, docTypeExpr(f.TypeExpr, entityNames), bufTag))
	}
	buf.WriteString("}\n\n")

	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		keyType, nestedEntity, ok := mapNestedEntity(f.TypeExpr, entityNames)
		if ok {
			stateMapType := fmt.Sprintf("map[%s]entity.%sState", keyType, nestedEntity)
			docMapType := fmt.Sprintf("map[%s]%sDoc", keyType, nestedEntity)
			buf.WriteString(fmt.Sprintf("func toDocMap_%s(in %s) %s {\n", f.RawName, stateMapType, docMapType))
			buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
			buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", docMapType))
			buf.WriteString("\tfor k, v := range in {\n")
			buf.WriteString(fmt.Sprintf("\t\tout[k] = %sStateToDoc(v)\n", nestedEntity))
			buf.WriteString("\t}\n")
			buf.WriteString("\treturn out\n")
			buf.WriteString("}\n\n")

			buf.WriteString(fmt.Sprintf("func toStateMap_%s(in %s) %s {\n", f.RawName, docMapType, stateMapType))
			buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
			buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateMapType))
			buf.WriteString("\tfor k, v := range in {\n")
			buf.WriteString(fmt.Sprintf("\t\tout[k] = %sDocToState(v)\n", nestedEntity))
			buf.WriteString("\t}\n")
			buf.WriteString("\treturn out\n")
			buf.WriteString("}\n\n")
			continue
		}
		if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
			if nestedEntity, nested := directNestedEntity(elemType, entityNames); nested {
				stateSliceType := fmt.Sprintf("[]entity.%sState", nestedEntity)
				docSliceType := fmt.Sprintf("[]%sDoc", nestedEntity)
				buf.WriteString(fmt.Sprintf("func toDocSlice_%s(in %s) %s {\n", f.RawName, stateSliceType, docSliceType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", docSliceType))
				buf.WriteString("\tfor i, v := range in {\n")
				buf.WriteString(fmt.Sprintf("\t\tout[i] = %sStateToDoc(v)\n", nestedEntity))
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")

				buf.WriteString(fmt.Sprintf("func toStateSlice_%s(in %s) %s {\n", f.RawName, docSliceType, stateSliceType))
				buf.WriteString("\tif in == nil {\n\t\treturn nil\n\t}\n")
				buf.WriteString(fmt.Sprintf("\tout := make(%s, len(in))\n", stateSliceType))
				buf.WriteString("\tfor i, v := range in {\n")
				buf.WriteString(fmt.Sprintf("\t\tout[i] = %sDocToState(v)\n", nestedEntity))
				buf.WriteString("\t}\n")
				buf.WriteString("\treturn out\n")
				buf.WriteString("}\n\n")
			}
		}
		if (f.Kind == kindMap || f.Kind == kindSlice) && containsNestedEntityType(f.TypeExpr, entityNames) {
			if _, _, ok := mapNestedEntity(f.TypeExpr, entityNames); ok {
				continue
			}
			if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
				if _, nested := directNestedEntity(elemType, entityNames); nested {
					continue
				}
			}
			if code, ok := genRecursiveConvertFunc("toDoc_"+f.RawName, f.TypeExpr, entityNames, recursiveConvStateToDoc, "entity.", "", ""); ok {
				buf.WriteString(code)
			}
			if code, ok := genRecursiveConvertFunc("toState_"+f.RawName, f.TypeExpr, entityNames, recursiveConvDocToState, "entity.", "", ""); ok {
				buf.WriteString(code)
			}
		}
	}

	buf.WriteString(fmt.Sprintf("func %sToDoc(s entity.%s) %s {\n", stateName, stateName, docName))
	buf.WriteString(fmt.Sprintf("\tstate := entity.Hydrate%sEntity(s).Save()\n", e.Name))
	buf.WriteString(fmt.Sprintf("\treturn %s{\n", docName))
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if _, _, ok := mapNestedEntity(f.TypeExpr, entityNames); ok {
			buf.WriteString(fmt.Sprintf("\t\t%s: toDocMap_%s(state.%s),\n", f.ExportName, f.RawName, f.ExportName))
			continue
		}
		if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
			if _, nested := directNestedEntity(elemType, entityNames); nested {
				buf.WriteString(fmt.Sprintf("\t\t%s: toDocSlice_%s(state.%s),\n", f.ExportName, f.RawName, f.ExportName))
				continue
			}
		}
		if (f.Kind == kindMap || f.Kind == kindSlice) && containsNestedEntityType(f.TypeExpr, entityNames) {
			buf.WriteString(fmt.Sprintf("\t\t%s: toDoc_%s(state.%s),\n", f.ExportName, f.RawName, f.ExportName))
			continue
		}
		if nestedEntity, ok := directNestedEntity(f.TypeExpr, entityNames); ok {
			buf.WriteString(fmt.Sprintf("\t\t%s: %sStateToDoc(state.%s),\n", f.ExportName, nestedEntity, f.ExportName))
			continue
		}
		buf.WriteString(fmt.Sprintf("\t\t%s: state.%s,\n", f.ExportName, f.ExportName))
	}
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func %sToState(d %s) entity.%s {\n", docName, docName, stateName))
	buf.WriteString(fmt.Sprintf("\tstate := entity.%s{\n", stateName))
	for _, f := range e.Fields {
		if f.Ignore {
			continue
		}
		if _, _, ok := mapNestedEntity(f.TypeExpr, entityNames); ok {
			buf.WriteString(fmt.Sprintf("\t\t%s: toStateMap_%s(d.%s),\n", f.ExportName, f.RawName, f.ExportName))
			continue
		}
		if elemType, ok := parseSliceElemType(f.TypeExpr); ok {
			if _, nested := directNestedEntity(elemType, entityNames); nested {
				buf.WriteString(fmt.Sprintf("\t\t%s: toStateSlice_%s(d.%s),\n", f.ExportName, f.RawName, f.ExportName))
				continue
			}
		}
		if (f.Kind == kindMap || f.Kind == kindSlice) && containsNestedEntityType(f.TypeExpr, entityNames) {
			buf.WriteString(fmt.Sprintf("\t\t%s: toState_%s(d.%s),\n", f.ExportName, f.RawName, f.ExportName))
			continue
		}
		if nestedEntity, ok := directNestedEntity(f.TypeExpr, entityNames); ok {
			buf.WriteString(fmt.Sprintf("\t\t%s: %sDocToState(d.%s),\n", f.ExportName, nestedEntity, f.ExportName))
			continue
		}
		buf.WriteString(fmt.Sprintf("\t\t%s: d.%s,\n", f.ExportName, f.ExportName))
	}
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\treturn entity.Hydrate%sEntity(state).Save()\n", e.Name))
	buf.WriteString("}\n")

	return writeGoFile(outPath, buf.Bytes())
}

// ---------- generation: mapper ----------

func genMappers(mapperPkg, modelPkg, entityPkg pkgInfo, entities []structInfo, outPath string) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen_entities; DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", mapperPkg.PkgName))

	buf.WriteString("import (\n")
	buf.WriteString(fmt.Sprintf("\tmodel %q\n", modelPkg.ImpPath))
	buf.WriteString(fmt.Sprintf("\tentity %q\n", entityPkg.ImpPath))
	buf.WriteString(")\n\n")

	for _, e := range entities {
		entityName := e.Name + "Entity"
		stateName := e.Name + "State"
		decodeHooks := collectHookMethods(e.Fields, true)
		encodeHooks := collectHookMethods(e.Fields, false)

		buf.WriteString(fmt.Sprintf("func %sModelToEntity(m *model.%s) *entity.%s {\n", e.Name, e.Name, entityName))
		buf.WriteString("\tif m == nil {\n\t\treturn nil\n\t}\n")
		buf.WriteString(fmt.Sprintf("\tvar s entity.%s\n", stateName))
		for _, f := range e.Fields {
			if f.Ignore {
				continue
			}
			buf.WriteString(fmt.Sprintf("\ts.%s = m.%s\n", f.ExportName, f.ResolvedModelName))
		}
		for _, methodName := range decodeHooks {
			buf.WriteString(fmt.Sprintf("\tif hook, ok := any(&s).(interface{ %s() }); ok {\n", methodName))
			buf.WriteString(fmt.Sprintf("\t\thook.%s()\n", methodName))
			buf.WriteString("\t}\n")
		}
		buf.WriteString(fmt.Sprintf("\treturn entity.Hydrate%s(s)\n", entityName))
		buf.WriteString("}\n\n")

		buf.WriteString(fmt.Sprintf("func %sEntityToModel(e *entity.%s) *model.%s {\n", e.Name, entityName, e.Name))
		buf.WriteString("\tif e == nil {\n\t\treturn nil\n\t}\n")
		buf.WriteString("\ts := e.Save()\n")
		for _, methodName := range encodeHooks {
			buf.WriteString(fmt.Sprintf("\tif hook, ok := any(&s).(interface{ %s() }); ok {\n", methodName))
			buf.WriteString(fmt.Sprintf("\t\thook.%s()\n", methodName))
			buf.WriteString("\t}\n")
		}
		buf.WriteString(fmt.Sprintf("\tout := &model.%s{}\n", e.Name))
		for _, f := range e.Fields {
			if f.Ignore {
				continue
			}
			buf.WriteString(fmt.Sprintf("\tout.%s = s.%s\n", f.ResolvedModelName, f.ExportName))
		}
		buf.WriteString("\treturn out\n")
		buf.WriteString("}\n\n")
	}

	return writeGoFile(outPath, buf.Bytes())
}

// ---------- helpers ----------

func collectEntityNames(entities []structInfo) map[string]bool {
	out := make(map[string]bool, len(entities))
	for i := range entities {
		out[entities[i].Name] = true
	}
	return out
}

func trimPointerType(typeExpr string) (base string, pointer bool) {
	t := strings.TrimSpace(typeExpr)
	for strings.HasPrefix(t, "*") {
		pointer = true
		t = strings.TrimSpace(strings.TrimPrefix(t, "*"))
	}
	return t, pointer
}

func directNestedEntity(typeExpr string, entityNames map[string]bool) (string, bool) {
	base, _ := trimPointerType(typeExpr)
	if entityNames[base] {
		return base, true
	}
	return "", false
}

func mapNestedEntity(typeExpr string, entityNames map[string]bool) (keyType, entityName string, ok bool) {
	keyType, valueType, ok := parseMapType(typeExpr)
	if !ok {
		return "", "", false
	}
	entityName, ok = directNestedEntity(valueType, entityNames)
	if !ok {
		return "", "", false
	}
	return keyType, entityName, true
}

type typeTransformMode int

const (
	typeTransformState typeTransformMode = iota
	typeTransformRuntime
	typeTransformDoc
)

func stateTypeExpr(typeExpr string, entityNames map[string]bool) string {
	return transformTypeExpr(typeExpr, entityNames, typeTransformState)
}

func runtimeTypeExpr(typeExpr string, entityNames map[string]bool) string {
	return transformTypeExpr(typeExpr, entityNames, typeTransformRuntime)
}

func docTypeExpr(typeExpr string, entityNames map[string]bool) string {
	return transformTypeExpr(typeExpr, entityNames, typeTransformDoc)
}

func transformTypeExpr(typeExpr string, entityNames map[string]bool, mode typeTransformMode) string {
	return transformTypeExprWithPrefix(typeExpr, entityNames, mode, "")
}

func transformTypeExprWithPrefix(typeExpr string, entityNames map[string]bool, mode typeTransformMode, entityPrefix string) string {
	node, ok := parseTypeExprNode(typeExpr)
	if !ok {
		return typeExpr
	}
	return node.renderWithPrefix(entityNames, mode, entityPrefix)
}

func containsNestedEntityType(typeExpr string, entityNames map[string]bool) bool {
	node, ok := parseTypeExprNode(typeExpr)
	if !ok {
		_, nested := directNestedEntity(typeExpr, entityNames)
		return nested
	}
	return node.containsNestedEntity(entityNames)
}

func containsNestedEntityInMapValue(typeExpr string, entityNames map[string]bool) bool {
	_, valueType, ok := parseMapType(typeExpr)
	if !ok {
		return false
	}
	return containsNestedEntityType(valueType, entityNames)
}

type typeExprNodeKind int

const (
	typeExprLeaf typeExprNodeKind = iota
	typeExprSlice
	typeExprMap
)

type typeExprNode struct {
	kind    typeExprNodeKind
	raw     string
	keyType string
	elem    *typeExprNode
}

func parseTypeExprNode(typeExpr string) (*typeExprNode, bool) {
	typeExpr = strings.TrimSpace(typeExpr)
	if typeExpr == "" {
		return nil, false
	}
	if strings.HasPrefix(typeExpr, "[]") {
		elem, ok := parseTypeExprNode(strings.TrimSpace(typeExpr[2:]))
		if !ok {
			return nil, false
		}
		return &typeExprNode{kind: typeExprSlice, elem: elem}, true
	}
	keyType, valueType, ok := parseMapType(typeExpr)
	if ok {
		elem, ok := parseTypeExprNode(valueType)
		if !ok {
			return nil, false
		}
		return &typeExprNode{kind: typeExprMap, keyType: keyType, elem: elem}, true
	}
	return &typeExprNode{kind: typeExprLeaf, raw: typeExpr}, true
}

func (n *typeExprNode) render(entityNames map[string]bool, mode typeTransformMode) string {
	return n.renderWithPrefix(entityNames, mode, "")
}

func (n *typeExprNode) renderWithPrefix(entityNames map[string]bool, mode typeTransformMode, entityPrefix string) string {
	if n == nil {
		return ""
	}
	switch n.kind {
	case typeExprSlice:
		return "[]" + n.elem.renderWithPrefix(entityNames, mode, entityPrefix)
	case typeExprMap:
		return fmt.Sprintf("map[%s]%s", n.keyType, n.elem.renderWithPrefix(entityNames, mode, entityPrefix))
	default:
		if entityName, ok := directNestedEntity(n.raw, entityNames); ok {
			switch mode {
			case typeTransformState:
				return entityPrefix + entityName + "State"
			case typeTransformRuntime:
				return "*" + entityPrefix + entityName + "Entity"
			case typeTransformDoc:
				return entityPrefix + entityName + "Doc"
			}
		}
		return n.raw
	}
}

func (n *typeExprNode) containsNestedEntity(entityNames map[string]bool) bool {
	if n == nil {
		return false
	}
	switch n.kind {
	case typeExprLeaf:
		_, ok := directNestedEntity(n.raw, entityNames)
		return ok
	case typeExprSlice, typeExprMap:
		return n.elem.containsNestedEntity(entityNames)
	default:
		return false
	}
}

type recursiveConvKind int

const (
	recursiveConvCloneState recursiveConvKind = iota
	recursiveConvStateToRuntime
	recursiveConvRuntimeToState
	recursiveConvStateToDoc
	recursiveConvDocToState
)

func recursiveConvModes(kind recursiveConvKind) (srcMode, dstMode typeTransformMode) {
	switch kind {
	case recursiveConvCloneState:
		return typeTransformState, typeTransformState
	case recursiveConvStateToRuntime:
		return typeTransformState, typeTransformRuntime
	case recursiveConvRuntimeToState:
		return typeTransformRuntime, typeTransformState
	case recursiveConvStateToDoc:
		return typeTransformState, typeTransformDoc
	case recursiveConvDocToState:
		return typeTransformDoc, typeTransformState
	default:
		return typeTransformState, typeTransformState
	}
}

func typePrefixForMode(mode typeTransformMode, statePrefix, runtimePrefix, docPrefix string) string {
	switch mode {
	case typeTransformState:
		return statePrefix
	case typeTransformRuntime:
		return runtimePrefix
	case typeTransformDoc:
		return docPrefix
	default:
		return ""
	}
}

func genRecursiveConvertFunc(funcName, typeExpr string, entityNames map[string]bool, kind recursiveConvKind, statePrefix, runtimePrefix, docPrefix string) (string, bool) {
	if !containsNestedEntityType(typeExpr, entityNames) {
		return "", false
	}
	node, ok := parseTypeExprNode(typeExpr)
	if !ok || node == nil {
		return "", false
	}
	srcMode, dstMode := recursiveConvModes(kind)
	srcType := node.renderWithPrefix(entityNames, srcMode, typePrefixForMode(srcMode, statePrefix, runtimePrefix, docPrefix))
	dstType := node.renderWithPrefix(entityNames, dstMode, typePrefixForMode(dstMode, statePrefix, runtimePrefix, docPrefix))

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("func %s(in %s) %s {\n", funcName, srcType, dstType))
	buf.WriteString(fmt.Sprintf("\tvar out %s\n", dstType))
	seq := 0
	emitRecursiveConvertAssign(&buf, "\t", "out", "in", node, entityNames, kind, statePrefix, runtimePrefix, docPrefix, &seq)
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")
	return buf.String(), true
}

func emitRecursiveConvertAssign(buf *bytes.Buffer, indent, dstExpr, srcExpr string, node *typeExprNode, entityNames map[string]bool, kind recursiveConvKind, statePrefix, runtimePrefix, docPrefix string, seq *int) {
	if node == nil {
		buf.WriteString(fmt.Sprintf("%s%s = %s\n", indent, dstExpr, srcExpr))
		return
	}
	_, dstMode := recursiveConvModes(kind)
	dstType := node.renderWithPrefix(entityNames, dstMode, typePrefixForMode(dstMode, statePrefix, runtimePrefix, docPrefix))

	switch node.kind {
	case typeExprLeaf:
		entityName, nested := directNestedEntity(node.raw, entityNames)
		if !nested {
			buf.WriteString(fmt.Sprintf("%s%s = %s\n", indent, dstExpr, srcExpr))
			return
		}
		switch kind {
		case recursiveConvCloneState:
			buf.WriteString(fmt.Sprintf("%s%s = %s\n", indent, dstExpr, srcExpr))
		case recursiveConvStateToRuntime:
			buf.WriteString(fmt.Sprintf("%s%s = Hydrate%sEntity(%s)\n", indent, dstExpr, entityName, srcExpr))
		case recursiveConvRuntimeToState:
			buf.WriteString(fmt.Sprintf("%sif %s == nil {\n", indent, srcExpr))
			buf.WriteString(fmt.Sprintf("%s\tvar z %s\n", indent, dstType))
			buf.WriteString(fmt.Sprintf("%s\t%s = z\n", indent, dstExpr))
			buf.WriteString(fmt.Sprintf("%s} else {\n", indent))
			buf.WriteString(fmt.Sprintf("%s\t%s = %s.Save()\n", indent, dstExpr, srcExpr))
			buf.WriteString(fmt.Sprintf("%s}\n", indent))
		case recursiveConvStateToDoc:
			buf.WriteString(fmt.Sprintf("%s%s = %sStateToDoc(%s)\n", indent, dstExpr, entityName, srcExpr))
		case recursiveConvDocToState:
			buf.WriteString(fmt.Sprintf("%s%s = %sDocToState(%s)\n", indent, dstExpr, entityName, srcExpr))
		default:
			buf.WriteString(fmt.Sprintf("%s%s = %s\n", indent, dstExpr, srcExpr))
		}
		return

	case typeExprSlice:
		buf.WriteString(fmt.Sprintf("%sif %s == nil {\n", indent, srcExpr))
		buf.WriteString(fmt.Sprintf("%s\t%s = nil\n", indent, dstExpr))
		buf.WriteString(fmt.Sprintf("%s} else {\n", indent))
		localOut := fmt.Sprintf("out%d", *seq)
		*seq++
		idx := fmt.Sprintf("i%d", *seq)
		*seq++
		val := fmt.Sprintf("v%d", *seq)
		*seq++
		buf.WriteString(fmt.Sprintf("%s\t%s := make(%s, len(%s))\n", indent, localOut, dstType, srcExpr))
		buf.WriteString(fmt.Sprintf("%s\tfor %s, %s := range %s {\n", indent, idx, val, srcExpr))
		emitRecursiveConvertAssign(buf, indent+"\t\t", fmt.Sprintf("%s[%s]", localOut, idx), val, node.elem, entityNames, kind, statePrefix, runtimePrefix, docPrefix, seq)
		buf.WriteString(fmt.Sprintf("%s\t}\n", indent))
		buf.WriteString(fmt.Sprintf("%s\t%s = %s\n", indent, dstExpr, localOut))
		buf.WriteString(fmt.Sprintf("%s}\n", indent))
		return

	case typeExprMap:
		buf.WriteString(fmt.Sprintf("%sif %s == nil {\n", indent, srcExpr))
		buf.WriteString(fmt.Sprintf("%s\t%s = nil\n", indent, dstExpr))
		buf.WriteString(fmt.Sprintf("%s} else {\n", indent))
		localOut := fmt.Sprintf("out%d", *seq)
		*seq++
		key := fmt.Sprintf("k%d", *seq)
		*seq++
		val := fmt.Sprintf("v%d", *seq)
		*seq++
		buf.WriteString(fmt.Sprintf("%s\t%s := make(%s, len(%s))\n", indent, localOut, dstType, srcExpr))
		buf.WriteString(fmt.Sprintf("%s\tfor %s, %s := range %s {\n", indent, key, val, srcExpr))
		emitRecursiveConvertAssign(buf, indent+"\t\t", fmt.Sprintf("%s[%s]", localOut, key), val, node.elem, entityNames, kind, statePrefix, runtimePrefix, docPrefix, seq)
		buf.WriteString(fmt.Sprintf("%s\t}\n", indent))
		buf.WriteString(fmt.Sprintf("%s\t%s = %s\n", indent, dstExpr, localOut))
		buf.WriteString(fmt.Sprintf("%s}\n", indent))
		return
	}

	buf.WriteString(fmt.Sprintf("%s%s = %s\n", indent, dstExpr, srcExpr))
}

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

func parseMapType(typeExpr string) (keyType string, valueType string, ok bool) {
	typeExpr = strings.TrimSpace(typeExpr)
	if !strings.HasPrefix(typeExpr, "map[") {
		return "", "", false
	}
	start := strings.Index(typeExpr, "[")
	if start < 0 {
		return "", "", false
	}
	depth := 0
	end := -1
	for i := start; i < len(typeExpr); i++ {
		switch typeExpr[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i
				i = len(typeExpr)
			}
		}
	}
	if end < 0 || end+1 >= len(typeExpr) {
		return "", "", false
	}
	keyType = strings.TrimSpace(typeExpr[start+1 : end])
	valueType = strings.TrimSpace(typeExpr[end+1:])
	if keyType == "" || valueType == "" {
		return "", "", false
	}
	return keyType, valueType, true
}

func parseSliceElemType(typeExpr string) (elemType string, ok bool) {
	typeExpr = strings.TrimSpace(typeExpr)
	if !strings.HasPrefix(typeExpr, "[]") {
		return "", false
	}
	elemType = strings.TrimSpace(typeExpr[2:])
	if elemType == "" {
		return "", false
	}
	return elemType, true
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

func toSnakeCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	out := make([]rune, 0, len(runes)+8)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := runes[i-1]
				var next rune
				hasNext := i+1 < len(runes)
				if hasNext {
					next = runes[i+1]
				}
				if (prev >= 'a' && prev <= 'z') || (hasNext && next >= 'a' && next <= 'z') {
					out = append(out, '_')
				}
			}
			r = r - 'A' + 'a'
		}
		out = append(out, r)
	}
	return string(out)
}
