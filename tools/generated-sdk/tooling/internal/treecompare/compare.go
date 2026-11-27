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

func comparePublicDeclarations(summary *Summary, path string, before, after []byte) {
	oldDeclarations := declarations(before)
	newDeclarations := declarations(after)
	for name, current := range newDeclarations {
		previous, ok := oldDeclarations[name]
		switch {
		case !ok:
			summary.PublicAdded = append(summary.PublicAdded, path+":"+name)
		case previous != current:
			summary.PublicChanged = append(summary.PublicChanged, path+":"+name)
		}
	}
	for name := range oldDeclarations {
		if _, ok := newDeclarations[name]; !ok {
			summary.PublicRemoved = append(summary.PublicRemoved, path+":"+name)
		}
	}
}

func declarations(data []byte) map[string]string {
	result := map[string]string{}
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
				result[kind+typed.Name.Name] = formatNode(&signature)
			}
		case *ast.GenDecl:
			for _, specification := range typed.Specs {
				switch item := specification.(type) {
				case *ast.TypeSpec:
					if item.Name.IsExported() {
						result["type "+item.Name.Name] = formatNode(item)
					}
				case *ast.ValueSpec:
					for _, name := range item.Names {
						if name.IsExported() {
							result[strings.ToLower(typed.Tok.String())+" "+name.Name] = formatNode(item)
						}
					}
				}
			}
		}
	}
	return result
}

func formatNode(node any) string {
	var output bytes.Buffer
	if err := format.Node(&output, token.NewFileSet(), node); err != nil {
		return fmt.Sprintf("%#v", node)
	}
	return output.String()
}
