package treecompare

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
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
	var diff bytes.Buffer
	paths := unionPaths(baseline, proposed)
	for _, path := range paths {
		before, hadBefore := baseline[path]
		after, hasAfter := proposed[path]
		switch {
		case !hadBefore:
			result.Summary.AddedFiles = append(result.Summary.AddedFiles, path)
			writeDiff(&diff, path, nil, after)
		case !hasAfter:
			result.Summary.RemovedFiles = append(result.Summary.RemovedFiles, path)
			writeDiff(&diff, path, before, nil)
		case !bytes.Equal(before, after):
			result.Summary.ChangedFiles = append(result.Summary.ChangedFiles, path)
			writeDiff(&diff, path, before, after)
		}
		if filepath.Ext(path) == ".go" {
			comparePublicDeclarations(&result.Summary, path, before, after)
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
	name      string
	node      ast.Node
	tok       token.Token
}

func comparePublicDeclarations(summary *Summary, path string, before, after []byte) {
	oldDeclarations := declarations(before)
	newDeclarations := declarations(after)
	for name, current := range newDeclarations {
		previous, ok := oldDeclarations[name]
		switch {
		case !ok:
			summary.PublicAdded = append(summary.PublicAdded, path+":"+name)
		case previous.formatted == current.formatted:
		case isAdditiveDeclarationEvolution(previous, current):
			summary.PublicAdditive = append(summary.PublicAdditive, path+":"+name)
		default:
			summary.PublicChanged = append(summary.PublicChanged, path+":"+name)
		}
	}
	for name := range oldDeclarations {
		if _, ok := newDeclarations[name]; !ok {
			summary.PublicRemoved = append(summary.PublicRemoved, path+":"+name)
		}
	}
}

func declarations(data []byte) map[string]declarationInfo {
	result := map[string]declarationInfo{}
	if len(data) == 0 {
		return result
	}
	file, err := parser.ParseFile(token.NewFileSet(), "", data, 0)
	if err != nil {
		return result
	}
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
					name:      typed.Name.Name,
					node:      &signature,
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
							name:      item.Name.Name,
							node:      item,
							tok:       token.TYPE,
						}
					}
				case *ast.ValueSpec:
					for _, name := range item.Names {
						if name.IsExported() {
							result[strings.ToLower(typed.Tok.String())+" "+name.Name] = declarationInfo{
								formatted: formatNode(item),
								name:      name.Name,
								node:      item,
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
	fieldType string
	tag       string
}

func isAdditiveDeclarationEvolution(previous, current declarationInfo) bool {
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
		if !exists || newField != oldField {
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
