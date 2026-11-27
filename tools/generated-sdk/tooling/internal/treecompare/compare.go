package treecompare

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/safetree"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
)

type Summary struct {
	AddedFiles         []string `json:"added_files"`
	ChangedFiles       []string `json:"changed_files"`
	PublicAdded        []string `json:"public_declarations_added"`
	PublicAdditive     []string `json:"public_declarations_additive"`
	PublicChanged      []string `json:"public_declarations_changed"`
	PublicRemoved      []string `json:"public_declarations_removed"`
	RemovedFiles       []string `json:"removed_files"`
	BaselineTreeSHA256 string   `json:"baseline_tree_sha256"`
	ProposedTreeSHA256 string   `json:"proposed_tree_sha256"`
}

type Result struct {
	Summary Summary
	Diff    []byte
}

func Compare(baselineRoot, proposedRoot string) (Result, error) {
	baseline, err := readTree(baselineRoot)
	if err != nil {
		return Result{}, err
	}
	proposed, err := readTree(proposedRoot)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Summary: Summary{
			AddedFiles:         []string{},
			ChangedFiles:       []string{},
			PublicAdded:        []string{},
			PublicAdditive:     []string{},
			PublicChanged:      []string{},
			PublicRemoved:      []string{},
			RemovedFiles:       []string{},
			BaselineTreeSHA256: treeDigest(baseline),
			ProposedTreeSHA256: treeDigest(proposed),
		},
	}
	baselineTrees := buildTreeIndex(baseline)
	proposedTrees := buildTreeIndex(proposed)
	var diff bytes.Buffer
	paths := unionPaths(baseline, proposed)
	for _, filePath := range paths {
		before, hadBefore := baseline[filePath]
		after, hasAfter := proposed[filePath]
		switch {
		case !hadBefore:
			result.Summary.AddedFiles = append(result.Summary.AddedFiles, filePath)
			writeDiff(&diff, filePath, nil, after)
		case !hasAfter:
			result.Summary.RemovedFiles = append(result.Summary.RemovedFiles, filePath)
			writeDiff(&diff, filePath, before, nil)
		case !bytes.Equal(before, after):
			result.Summary.ChangedFiles = append(result.Summary.ChangedFiles, filePath)
			writeDiff(&diff, filePath, before, after)
		}
		if filepath.Ext(filePath) == ".go" {
			comparePublicDeclarations(&result.Summary, filePath, baselineTrees, proposedTrees)
		}
	}
	slices.Sort(result.Summary.PublicAdded)
	slices.Sort(result.Summary.PublicAdditive)
	slices.Sort(result.Summary.PublicChanged)
	slices.Sort(result.Summary.PublicRemoved)
	result.Diff = diff.Bytes()
	return result, nil
}

func readTree(root string) (map[string][]byte, error) {
	if err := safetree.Validate(root); err != nil {
		return nil, err
	}
	result := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := safetree.ReadRegular(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = data
		return nil
	})
	return result, err
}

func treeDigest(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	var content bytes.Buffer
	for _, path := range paths {
		content.WriteString(path)
		content.WriteByte(0)
		content.WriteString(spec.Digest(files[path]))
		content.WriteByte('\n')
	}
	return spec.Digest(content.Bytes())
}

func unionPaths(left, right map[string][]byte) []string {
	seen := map[string]bool{}
	for path := range left {
		seen[path] = true
	}
	for path := range right {
		seen[path] = true
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func writeDiff(output *bytes.Buffer, path string, before, after []byte) {
	fmt.Fprintf(output, "--- pinned/%s\n+++ proposed/%s\n", path, path)
	for _, line := range splitLines(before) {
		output.WriteByte('-')
		output.WriteString(line)
		output.WriteByte('\n')
	}
	for _, line := range splitLines(after) {
		output.WriteByte('+')
		output.WriteString(line)
		output.WriteByte('\n')
	}
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

type declarationInfo struct {
	formatted string
	imports   map[string]string
	name      string
	node      ast.Node
	pkgKey    string
	tok       token.Token
}

type packageTypeDef struct {
	imports map[string]string
	spec    *ast.TypeSpec
}

type treeIndex struct {
	declarationsByFile map[string]map[string]declarationInfo
	typesByPackage     map[string]map[string]packageTypeDef
}

func buildTreeIndex(files map[string][]byte) *treeIndex {
	index := &treeIndex{
		declarationsByFile: make(map[string]map[string]declarationInfo, len(files)),
		typesByPackage:     map[string]map[string]packageTypeDef{},
	}
	for filePath, data := range files {
		if filepath.Ext(filePath) != ".go" || len(data) == 0 {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filePath, data, 0)
		if err != nil {
			continue
		}
		pkgKey := path.Dir(filePath) + "\x00" + parsed.Name.Name
		imports := fileImports(parsed)
		pkgTypes := index.typesByPackage[pkgKey]
		if pkgTypes == nil {
			pkgTypes = map[string]packageTypeDef{}
			index.typesByPackage[pkgKey] = pkgTypes
		}
		for _, declaration := range parsed.Decls {
			genDecl, ok := declaration.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, specNode := range genDecl.Specs {
				typeSpec, ok := specNode.(*ast.TypeSpec)
				if !ok {
					continue
				}
				pkgTypes[typeSpec.Name.Name] = packageTypeDef{
					imports: imports,
					spec:    typeSpec,
				}
			}
		}
		index.declarationsByFile[filePath] = fileDeclarations(parsed, pkgKey, imports)
	}
	return index
}

func fileImports(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, specNode := range file.Imports {
		importPath, err := strconv.Unquote(specNode.Path.Value)
		if err != nil {
			continue
		}
		localName := path.Base(importPath)
		if specNode.Name != nil {
			if specNode.Name.Name == "_" || specNode.Name.Name == "." {
				continue
			}
			localName = specNode.Name.Name
		}
		imports[localName] = importPath
	}
	return imports
}

func comparePublicDeclarations(summary *Summary, filePath string, baseline, proposed *treeIndex) {
	oldDeclarations := baseline.declarationsByFile[filePath]
	newDeclarations := proposed.declarationsByFile[filePath]
	for name, current := range newDeclarations {
		previous, ok := oldDeclarations[name]
		switch {
		case !ok:
			summary.PublicAdded = append(summary.PublicAdded, filePath+":"+name)
		case previous.formatted == current.formatted:
			if declarationLosesComparability(previous, current, baseline, proposed) {
				summary.PublicChanged = append(summary.PublicChanged, filePath+":"+name)
			}
		case isAdditiveDeclarationEvolution(previous, current, baseline, proposed):
			summary.PublicAdditive = append(summary.PublicAdditive, filePath+":"+name)
		default:
			summary.PublicChanged = append(summary.PublicChanged, filePath+":"+name)
		}
	}
	for name := range oldDeclarations {
		if _, ok := newDeclarations[name]; !ok {
			summary.PublicRemoved = append(summary.PublicRemoved, filePath+":"+name)
		}
	}
}

func fileDeclarations(file *ast.File, pkgKey string, imports map[string]string) map[string]declarationInfo {
	result := map[string]declarationInfo{}
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if typed.Name.IsExported() {
				kind := "func "
				if typed.Recv != nil && len(typed.Recv.List) != 0 {
					kind = "method " + formatNode(typed.Recv.List[0].Type) + "."
				}
				signature := *typed
				signature.Body = nil
				result[kind+typed.Name.Name] = declarationInfo{
					formatted: formatNode(&signature),
					imports:   imports,
					name:      typed.Name.Name,
					node:      &signature,
					pkgKey:    pkgKey,
					tok:       token.FUNC,
				}
			}
		case *ast.GenDecl:
			for _, specification := range typed.Specs {
				switch item := specification.(type) {
				case *ast.TypeSpec:
					if item.Name.IsExported() {
						result["type "+item.Name.Name] = declarationInfo{
							formatted: formatNode(item),
							imports:   imports,
							name:      item.Name.Name,
							node:      item,
							pkgKey:    pkgKey,
							tok:       token.TYPE,
						}
					}
				case *ast.ValueSpec:
					for _, name := range item.Names {
						if name.IsExported() {
							result[strings.ToLower(typed.Tok.String())+" "+name.Name] = declarationInfo{
								formatted: formatNode(item),
								imports:   imports,
								name:      name.Name,
								node:      item,
								pkgKey:    pkgKey,
								tok:       typed.Tok,
							}
						}
					}
				}
			}
		}
	}
	return result
}

type structFieldInfo struct {
	embedded  bool
	exported  bool
	expr      ast.Expr
	fieldType string
	tag       string
}

func isAdditiveDeclarationEvolution(previous, current declarationInfo, baseline, proposed *treeIndex) bool {
	if previous.tok != current.tok {
		return false
	}
	switch previous.tok {
	case token.TYPE:
		oldSpec, oldOK := previous.node.(*ast.TypeSpec)
		newSpec, newOK := current.node.(*ast.TypeSpec)
		if !oldOK || !newOK {
			return false
		}
		if declarationLosesComparability(previous, current, baseline, proposed) {
			return false
		}
		return isAdditiveStructEvolution(oldSpec, newSpec)
	case token.VAR:
		oldSpec, oldOK := previous.node.(*ast.ValueSpec)
		newSpec, newOK := current.node.(*ast.ValueSpec)
		if !oldOK || !newOK {
			return false
		}
		return isAdditiveAllowedEnumValuesEvolution(previous.name, oldSpec, newSpec)
	default:
		return false
	}
}

func isAdditiveStructEvolution(oldSpec, newSpec *ast.TypeSpec) bool {
	if oldSpec.Assign.IsValid() || newSpec.Assign.IsValid() {
		return false
	}
	if formatOptionalNode(oldSpec.TypeParams) != formatOptionalNode(newSpec.TypeParams) {
		return false
	}
	oldStruct, oldOK := oldSpec.Type.(*ast.StructType)
	newStruct, newOK := newSpec.Type.(*ast.StructType)
	if !oldOK || !newOK {
		return false
	}
	oldFields, oldHasUnexported := extractStructFields(oldStruct)
	newFields, _ := extractStructFields(newStruct)
	if len(newFields) <= len(oldFields) {
		return false
	}
	for key, oldField := range oldFields {
		newField, exists := newFields[key]
		if !exists ||
			newField.embedded != oldField.embedded ||
			newField.exported != oldField.exported ||
			newField.fieldType != oldField.fieldType ||
			newField.tag != oldField.tag {
			return false
		}
	}
	for key, newField := range newFields {
		if _, existed := oldFields[key]; existed {
			continue
		}
		if newField.embedded {
			return false
		}
		if !newField.exported && !oldHasUnexported {
			return false
		}
	}
	return true
}

type comparabilityKind int

const (
	compFalse comparabilityKind = iota
	compUnknown
	compInterface
	compStrict
)

func declarationLosesComparability(previous, current declarationInfo, baseline, proposed *treeIndex) bool {
	if previous.tok != token.TYPE || current.tok != token.TYPE {
		return false
	}
	oldSpec, oldOK := previous.node.(*ast.TypeSpec)
	newSpec, newOK := current.node.(*ast.TypeSpec)
	if !oldOK || !newOK {
		return false
	}
	oldPkg := baseline.typesByPackage[previous.pkgKey]
	newPkg := proposed.typesByPackage[current.pkgKey]
	oldStruct, oldIsStruct := oldSpec.Type.(*ast.StructType)
	newStruct, newIsStruct := newSpec.Type.(*ast.StructType)
	if oldIsStruct && newIsStruct && !oldSpec.Assign.IsValid() && !newSpec.Assign.IsValid() {
		return structLosesComparability(oldStruct, previous.imports, oldPkg, newStruct, current.imports, newPkg)
	}
	oldComp := exprComparability(oldSpec.Type, previous.imports, oldPkg, map[string]bool{oldSpec.Name.Name: true})
	newComp := exprComparability(newSpec.Type, current.imports, newPkg, map[string]bool{newSpec.Name.Name: true})
	return comparabilityDegraded(oldComp, newComp)
}

func structLosesComparability(
	oldStruct *ast.StructType,
	oldImports map[string]string,
	oldPkg map[string]packageTypeDef,
	newStruct *ast.StructType,
	newImports map[string]string,
	newPkg map[string]packageTypeDef,
) bool {
	oldFields, _ := extractStructFields(oldStruct)
	newFields, _ := extractStructFields(newStruct)
	oldOverall := compStrict
	oldFieldComp := make(map[string]comparabilityKind, len(oldFields))
	for key, field := range oldFields {
		comp := exprComparability(field.expr, oldImports, oldPkg, map[string]bool{})
		oldFieldComp[key] = comp
		oldOverall = combineComparability(oldOverall, comp)
	}
	if oldOverall == compFalse {
		return false
	}
	newOverall := compStrict
	for key, field := range newFields {
		comp := exprComparability(field.expr, newImports, newPkg, map[string]bool{})
		if oldField, existed := oldFields[key]; existed &&
			oldField.fieldType == field.fieldType &&
			oldFieldComp[key] == compUnknown &&
			comp == compUnknown {
			// An unchanged unresolved external field contributes the same unknown comparability on both sides;
			// still require every newly added or changed field to preserve comparability.
			continue
		}
		newOverall = combineComparability(newOverall, comp)
	}
	return comparabilityDegraded(oldOverall, newOverall)
}

func comparabilityDegraded(before, after comparabilityKind) bool {
	if before == compFalse {
		return false
	}
	// If baseline could be comparable (compStrict, compInterface, or unknown external type),
	// proposed must be provably comparable (not compFalse and not compUnknown).
	if after == compFalse || after == compUnknown {
		return true
	}
	// If baseline had panic-free concrete comparability (compStrict), adding an interface field
	// degrades == to a potential runtime panic on dynamic slice/map values.
	if before == compStrict && after == compInterface {
		return true
	}
	return false
}

func combineComparability(left, right comparabilityKind) comparabilityKind {
	if left < right {
		return left
	}
	return right
}

func exprComparability(
	expr ast.Expr,
	imports map[string]string,
	pkgTypes map[string]packageTypeDef,
	visiting map[string]bool,
) comparabilityKind {
	switch typed := expr.(type) {
	case *ast.ParenExpr:
		return exprComparability(typed.X, imports, pkgTypes, visiting)
	case *ast.StarExpr, *ast.ChanType:
		return compStrict
	case *ast.MapType, *ast.FuncType:
		return compFalse
	case *ast.ArrayType:
		if typed.Len == nil {
			return compFalse
		}
		return exprComparability(typed.Elt, imports, pkgTypes, visiting)
	case *ast.InterfaceType:
		return compInterface
	case *ast.StructType:
		overall := compStrict
		if typed.Fields == nil {
			return overall
		}
		for _, field := range typed.Fields.List {
			overall = combineComparability(overall, exprComparability(field.Type, imports, pkgTypes, visiting))
			if overall == compFalse {
				return compFalse
			}
		}
		return overall
	case *ast.Ident:
		if def, exists := pkgTypes[typed.Name]; exists {
			if visiting[typed.Name] {
				return compUnknown
			}
			if def.spec.TypeParams != nil && len(def.spec.TypeParams.List) > 0 {
				return compUnknown
			}
			nextVisiting := make(map[string]bool, len(visiting)+1)
			for k, v := range visiting {
				nextVisiting[k] = v
			}
			nextVisiting[typed.Name] = true
			return exprComparability(def.spec.Type, def.imports, pkgTypes, nextVisiting)
		}
		return predeclaredComparability(typed.Name)
	case *ast.SelectorExpr:
		pkgIdent, ok := typed.X.(*ast.Ident)
		if !ok {
			return compUnknown
		}
		importPath, ok := imports[pkgIdent.Name]
		if !ok {
			importPath = pkgIdent.Name
		}
		return stdlibComparability(importPath, typed.Sel.Name)
	default:
		return compUnknown
	}
}

func predeclaredComparability(name string) comparabilityKind {
	switch name {
	case "bool", "byte", "rune",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"string":
		return compStrict
	case "any", "error", "comparable":
		return compInterface
	default:
		return compUnknown
	}
}

func stdlibComparability(importPath, typeName string) comparabilityKind {
	switch importPath + "." + typeName {
	case "time.Time", "net/url.URL":
		return compStrict
	case "encoding/json.RawMessage",
		"net/http.Header", "net/http.Request", "net/http.Response",
		"net/url.Values", "bytes.Buffer", "regexp.Regexp",
		"mime/multipart.FileHeader":
		return compFalse
	case "context.Context",
		"io.Reader", "io.Writer", "io.Closer", "io.ReadCloser", "io.ReadWriter",
		"net/http.RoundTripper", "mime/multipart.File":
		return compInterface
	default:
		return compUnknown
	}
}

func extractStructFields(st *ast.StructType) (map[string]structFieldInfo, bool) {
	fields := map[string]structFieldInfo{}
	hasUnexported := false
	if st == nil || st.Fields == nil {
		return fields, false
	}
	for _, field := range st.Fields.List {
		fieldType := formatNode(field.Type)
		tag := ""
		if field.Tag != nil {
			tag = field.Tag.Value
		}
		if len(field.Names) == 0 {
			exported := embeddedTypeExported(field.Type)
			if !exported {
				hasUnexported = true
			}
			fields["embedded:"+fieldType] = structFieldInfo{
				embedded:  true,
				exported:  exported,
				expr:      field.Type,
				fieldType: fieldType,
				tag:       tag,
			}
			continue
		}
		for _, ident := range field.Names {
			exported := ident.IsExported()
			if !exported {
				hasUnexported = true
			}
			fields["field:"+ident.Name] = structFieldInfo{
				exported:  exported,
				expr:      field.Type,
				fieldType: fieldType,
				tag:       tag,
			}
		}
	}
	return fields, hasUnexported
}

func embeddedTypeExported(expr ast.Expr) bool {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.IsExported()
	case *ast.SelectorExpr:
		return typed.Sel.IsExported()
	case *ast.IndexExpr:
		return embeddedTypeExported(typed.X)
	case *ast.IndexListExpr:
		return embeddedTypeExported(typed.X)
	default:
		return false
	}
}

func isAdditiveAllowedEnumValuesEvolution(name string, oldSpec, newSpec *ast.ValueSpec) bool {
	if !strings.HasPrefix(name, "Allowed") || !strings.HasSuffix(name, "EnumValues") {
		return false
	}
	if len(oldSpec.Names) != 1 || len(newSpec.Names) != 1 || len(oldSpec.Values) != 1 || len(newSpec.Values) != 1 {
		return false
	}
	if formatOptionalNode(oldSpec.Type) != formatOptionalNode(newSpec.Type) {
		return false
	}
	oldLit, oldOK := oldSpec.Values[0].(*ast.CompositeLit)
	newLit, newOK := newSpec.Values[0].(*ast.CompositeLit)
	if !oldOK || !newOK {
		return false
	}
	oldSlice, oldOK := oldLit.Type.(*ast.ArrayType)
	newSlice, newOK := newLit.Type.(*ast.ArrayType)
	if !oldOK || !newOK || oldSlice.Len != nil || newSlice.Len != nil {
		return false
	}
	if formatNode(oldLit.Type) != formatNode(newLit.Type) {
		return false
	}
	oldElements := map[string]bool{}
	for _, elt := range oldLit.Elts {
		oldElements[formatNode(elt)] = true
	}
	newElements := map[string]bool{}
	for _, elt := range newLit.Elts {
		newElements[formatNode(elt)] = true
	}
	if len(newElements) <= len(oldElements) {
		return false
	}
	for element := range oldElements {
		if !newElements[element] {
			return false
		}
	}
	return true
}

func formatOptionalNode(node ast.Node) string {
	if node == nil {
		return ""
	}
	switch typed := node.(type) {
	case *ast.FieldList:
		if typed == nil {
			return ""
		}
	}
	return formatNode(node)
}

func formatNode(node any) string {
	var output bytes.Buffer
	if err := format.Node(&output, token.NewFileSet(), node); err != nil {
		return fmt.Sprintf("%#v", node)
	}
	return output.String()
}
