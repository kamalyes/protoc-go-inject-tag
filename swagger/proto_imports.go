package swagger

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var protoImportLineRegexp = regexp.MustCompile(`^\s*import\s+(?:public\s+)?"([^"]+)"\s*;`)

// ExpandProtoFilesWithImports 递归解析 import，合并依赖的 proto 文件（用于跨模块 message 字段索引）
func ExpandProtoFilesWithImports(protoFiles []string) ([]string, error) {
	if len(protoFiles) == 0 {
		return nil, nil
	}
	root, ok := findProtoRepoRoot(protoFiles)
	if !ok {
		return dedupeStrings(protoFiles), nil
	}

	seen := make(map[string]bool)
	var ordered []string
	queue := append([]string{}, protoFiles...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		abs, err := filepath.Abs(current)
		if err != nil {
			abs = current
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		ordered = append(ordered, abs)

		imports, err := parseProtoImportPaths(abs)
		if err != nil {
			return nil, err
		}
		for _, imp := range imports {
			resolved := filepath.Join(root, filepath.FromSlash(imp))
			if _, statErr := os.Stat(resolved); statErr != nil {
				continue
			}
			queue = append(queue, resolved)
		}
	}
	return ordered, nil
}

func findProtoRepoRoot(protoFiles []string) (string, bool) {
	for _, f := range protoFiles {
		abs, err := filepath.Abs(f)
		if err != nil {
			abs = f
		}
		idx := strings.Index(abs, string(filepath.Separator)+"proto"+string(filepath.Separator))
		if idx >= 0 {
			return abs[:idx], true
		}
	}
	return "", false
}

func parseProtoImportPaths(protoFile string) ([]string, error) {
	f, err := os.Open(protoFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var imports []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := protoImportLineRegexp.FindStringSubmatch(scanner.Text())
		if len(m) == 2 {
			imports = append(imports, m[1])
		}
	}
	return imports, scanner.Err()
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
