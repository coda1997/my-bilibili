package check

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

var workflowFixedFiles = []string{
	"README.md",
	"AGENTS.md",
	"STATUS.md",
	".github/copilot-instructions.md",
	".github/pull_request_template.md",
	"docs/agent-convergence.md",
	"docs/architecture.md",
	"docs/development.md",
	"docs/pull-requests.md",
	"docs/testing.md",
	"docs/workflow.md",
}

var (
	workflowMarkdownParser = goldmark.DefaultParser()
	bareRouteToken         = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	windowsDrivePath       = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

type markdownDocument struct {
	source     []byte
	root       ast.Node
	references []parser.Reference
}

type workflowLinkContext struct {
	root        string
	documents   map[string]markdownDocument
	anchors     map[string]map[string]bool
	resolutions map[string]pathResolution
}

type pathResolution struct {
	path string
	err  error
}

func Workflow(root string) (int, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return 0, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return 0, err
	}
	files, skills, err := workflowInventory(root)
	if err != nil {
		return 0, err
	}
	documents := make(map[string]markdownDocument, len(files))
	for _, file := range files {
		data, err := readWorkflowFile(root, file)
		if err != nil {
			return 0, err
		}
		documents[file] = parseMarkdownDocument(data)
	}
	agents, ok := documents["AGENTS.md"]
	if !ok {
		return 0, errors.New("workflow inventory omitted AGENTS.md")
	}
	if err := validateSkillRoutes(agents, skills); err != nil {
		return 0, err
	}
	linkContext := &workflowLinkContext{
		root:        root,
		documents:   documents,
		anchors:     make(map[string]map[string]bool),
		resolutions: make(map[string]pathResolution),
	}
	for _, file := range files {
		if err := validateMarkdownLinks(linkContext, file, documents[file]); err != nil {
			return 0, err
		}
	}
	return len(files), nil
}

func workflowInventory(root string) ([]string, map[string]string, error) {
	seen := make(map[string]bool)
	var files []string
	add := func(relative string) {
		relative = filepath.ToSlash(relative)
		if !seen[relative] {
			seen[relative] = true
			files = append(files, relative)
		}
	}
	for _, relative := range workflowFixedFiles {
		add(relative)
	}

	entries, err := readWorkflowDir(filepath.Join(root, ".agents", "skills"), ".agents/skills")
	if err != nil {
		return nil, nil, err
	}
	if len(entries) == 0 {
		return nil, nil, errors.New(".agents/skills: no skills found")
	}
	skills := make(map[string]string, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		isSkill, err := classifySkillEntry(name, entry.Type(), entry.IsDir())
		if err != nil {
			return nil, nil, err
		}
		if !isSkill {
			continue
		}
		relative := filepath.ToSlash(filepath.Join(".agents/skills", name, "SKILL.md"))
		data, err := readWorkflowFile(root, relative)
		if err != nil {
			return nil, nil, err
		}
		frontmatterName, description, err := skillFrontmatter(data)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", relative, err)
		}
		if frontmatterName != name {
			return nil, nil, fmt.Errorf("%s: skill name %q must equal directory name %q", relative, frontmatterName, name)
		}
		if description == "" {
			return nil, nil, fmt.Errorf("%s: skill description is empty", relative)
		}
		if previous, exists := skills[frontmatterName]; exists {
			return nil, nil, fmt.Errorf("%s: duplicate skill name %q also used by %s", relative, frontmatterName, previous)
		}
		skills[frontmatterName] = relative
		add(relative)
	}
	if len(skills) == 0 {
		return nil, nil, errors.New(".agents/skills: no skill directories found")
	}

	templates, err := readWorkflowDir(filepath.Join(root, "docs", "templates"), "docs/templates")
	if err != nil {
		return nil, nil, err
	}
	templateCount := 0
	for _, entry := range templates {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, nil, fmt.Errorf("docs/templates/%s: symlink is not allowed", entry.Name())
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			add(filepath.Join("docs/templates", entry.Name()))
			templateCount++
		}
	}
	if templateCount == 0 {
		return nil, nil, errors.New("docs/templates: no Markdown templates found")
	}
	sort.Strings(files)
	return files, skills, nil
}

func readWorkflowDir(path, relative string) ([]os.DirEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("%s: workflow root must be a directory, not a symlink", relative)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	return entries, nil
}

func readWorkflowFile(root, relative string) ([]byte, error) {
	if !fs.ValidPath(relative) {
		return nil, fmt.Errorf("%s: invalid repository path", relative)
	}
	current := root
	parts := strings.Split(relative, "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relative, err)
		}
		prefix := filepath.ToSlash(strings.Join(parts[:i+1], "/"))
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s: symlink is not allowed in workflow inputs", prefix)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("%s: workflow parent is not a directory", prefix)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: workflow input is not a regular file", relative)
		}
	}
	data, err := os.ReadFile(current)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	return data, nil
}

func skillFrontmatter(data []byte) (string, string, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return "", "", errors.New("missing YAML frontmatter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", "", errors.New("unterminated YAML frontmatter")
	}
	var metadata struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	frontmatter := []byte(strings.Join(lines[1:end], "\n"))
	if err := yaml.Unmarshal(frontmatter, &metadata); err != nil {
		return "", "", fmt.Errorf("invalid YAML frontmatter: %w", err)
	}
	if strings.TrimSpace(metadata.Name) == "" {
		return "", "", errors.New("missing non-empty name frontmatter key")
	}
	if strings.TrimSpace(metadata.Description) == "" {
		return "", "", errors.New("missing non-empty description frontmatter key")
	}
	return metadata.Name, metadata.Description, nil
}

func parseMarkdownDocument(source []byte) markdownDocument {
	context := parser.NewContext()
	root := workflowMarkdownParser.Parse(text.NewReader(source), parser.WithContext(context))
	return markdownDocument{
		source:     source,
		root:       root,
		references: context.References(),
	}
}

func validateSkillRoutes(document markdownDocument, skills map[string]string) error {
	sectionLevel := 0
	inSection := false
	counts := make(map[string]int)
	found := false
	for node := document.root.FirstChild(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*ast.Heading); ok {
			title, err := nodePlainText(heading, document.source)
			if err != nil {
				return err
			}
			if !inSection && strings.EqualFold(strings.TrimSpace(title), "Task routing") {
				inSection = true
				sectionLevel = heading.Level
				found = true
				continue
			}
			if inSection && heading.Level <= sectionLevel {
				inSection = false
			}
		}
		if !inSection {
			continue
		}
		err := ast.Walk(node, func(current ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			link, ok := current.(*ast.Link)
			if !ok || !insideListItem(link) {
				return ast.WalkContinue, nil
			}
			name, isRoute, err := routeLinkName(link, document.source)
			if err != nil {
				return ast.WalkStop, err
			}
			if !isRoute {
				return ast.WalkContinue, nil
			}
			relative, exists := skills[name]
			if !exists {
				return ast.WalkStop, fmt.Errorf("AGENTS.md: routed skill %q has no .agents/skills/%s/SKILL.md", name, name)
			}
			wantTarget := ".agents/skills/" + name + "/SKILL.md"
			if string(link.Destination) != wantTarget {
				return ast.WalkStop, fmt.Errorf("AGENTS.md: routed skill %q must link to %s", name, wantTarget)
			}
			counts[name]++
			if counts[name] > 1 {
				return ast.WalkStop, fmt.Errorf("AGENTS.md: routed skill %q appears more than once (%s)", name, relative)
			}
			return ast.WalkContinue, nil
		})
		if err != nil {
			return err
		}
	}
	if !found {
		return errors.New("AGENTS.md: missing Task routing section")
	}
	for name, relative := range skills {
		if counts[name] != 1 {
			return fmt.Errorf("AGENTS.md: skill %q (%s) must appear exactly once in Task routing", name, relative)
		}
	}
	return nil
}

func validateMarkdownLinks(context *workflowLinkContext, relative string, document markdownDocument) error {
	for _, reference := range document.references {
		label := string(reference.Label())
		if strings.HasPrefix(label, "^") {
			continue
		}
		if err := validateMarkdownTarget(context, relative, label, string(reference.Destination())); err != nil {
			return err
		}
	}
	return ast.Walk(document.root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch link := node.(type) {
		case *ast.Link:
			if err := validateMarkdownTarget(context, relative, string(link.Text(document.source)), string(link.Destination)); err != nil {
				return ast.WalkStop, err
			}
		case *ast.Image:
			if err := validateMarkdownTarget(context, relative, string(link.Text(document.source)), string(link.Destination)); err != nil {
				return ast.WalkStop, err
			}
		}
		return ast.WalkContinue, nil
	})
}

func validateMarkdownTarget(context *workflowLinkContext, relative, label, target string) error {
	if isWindowsLocalPath(target) {
		return fmt.Errorf("%s: link %q is a Windows local absolute path outside the repository", relative, target)
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("%s: invalid Markdown link %q: %w", relative, target, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(target, "//") {
		return nil
	}
	if filepath.IsAbs(filepath.FromSlash(parsed.Path)) {
		return fmt.Errorf("%s: absolute repository link %q is not allowed", relative, target)
	}
	targetPath := relative
	if parsed.Path != "" {
		targetPath = filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(relative)), filepath.FromSlash(parsed.Path)))
	}
	resolution, ok := context.resolutions[targetPath]
	if !ok {
		resolution.path, resolution.err = resolveInsideRoot(context.root, targetPath)
		context.resolutions[targetPath] = resolution
	}
	if resolution.err != nil {
		return fmt.Errorf("%s: %w", relative, resolution.err)
	}
	info, err := os.Stat(resolution.path)
	if err != nil {
		return fmt.Errorf("%s: reference %q link %q does not resolve: %w", relative, label, target, err)
	}
	if parsed.Fragment == "" {
		return nil
	}
	if info.IsDir() || !strings.EqualFold(filepath.Ext(resolution.path), ".md") {
		return fmt.Errorf("%s: link %q has a fragment but target is not Markdown", relative, target)
	}
	anchors, err := context.markdownAnchors(targetPath, resolution.path)
	if err != nil {
		return fmt.Errorf("%s: read fragment target %q: %w", relative, target, err)
	}
	fragment := strings.ToLower(parsed.Fragment)
	if !anchors[fragment] {
		return fmt.Errorf("%s: link %q names missing Markdown heading %q", relative, target, parsed.Fragment)
	}
	return nil
}

func (context *workflowLinkContext) markdownAnchors(relative, resolved string) (map[string]bool, error) {
	if anchors, ok := context.anchors[resolved]; ok {
		return anchors, nil
	}
	document, ok := context.documents[relative]
	if !ok {
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, err
		}
		document = parseMarkdownDocument(data)
		context.documents[relative] = document
	}
	anchors, err := markdownAnchors(document)
	if err != nil {
		return nil, err
	}
	context.anchors[resolved] = anchors
	return anchors, nil
}

func markdownAnchors(document markdownDocument) (map[string]bool, error) {
	anchors := make(map[string]bool)
	err := ast.Walk(document.root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		heading, ok := node.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		text, err := nodePlainText(heading, document.source)
		if err != nil {
			return ast.WalkStop, err
		}
		base := markdownSlug(text)
		if base == "" {
			return ast.WalkContinue, nil
		}
		anchor := base
		for suffix := 1; anchors[anchor]; suffix++ {
			anchor = fmt.Sprintf("%s-%d", base, suffix)
		}
		anchors[anchor] = true
		return ast.WalkSkipChildren, nil
	})
	return anchors, err
}

func markdownSlug(value string) string {
	var slug strings.Builder
	value = strings.TrimSpace(strings.ToLower(value))
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			slug.WriteRune(r)
		case unicode.IsSpace(r):
			slug.WriteByte('-')
		}
	}
	return slug.String()
}

func nodePlainText(node ast.Node, source []byte) (string, error) {
	var value strings.Builder
	err := walkNodeText(node, source, func(_ ast.Node, text []byte) error {
		value.Write(text)
		return nil
	})
	return value.String(), err
}

func walkNodeText(node ast.Node, source []byte, visit func(ast.Node, []byte) error) error {
	return ast.Walk(node, func(current ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var text []byte
		switch current := current.(type) {
		case *ast.Text:
			text = current.Segment.Value(source)
		case *ast.String:
			text = current.Value
		case *ast.RawHTML:
			text = current.Segments.Value(source)
		default:
			return ast.WalkContinue, nil
		}
		if err := visit(current, text); err != nil {
			return ast.WalkStop, err
		}
		return ast.WalkContinue, nil
	})
}

func routeLinkName(link *ast.Link, source []byte) (string, bool, error) {
	child := link.FirstChild()
	if child == nil || child.NextSibling() != nil {
		return "", false, nil
	}
	if _, ok := child.(*ast.CodeSpan); !ok {
		return "", false, nil
	}
	text, err := nodePlainText(child, source)
	if err != nil {
		return "", false, err
	}
	name := strings.TrimSpace(text)
	if !bareRouteToken.MatchString(name) {
		return "", false, nil
	}
	return name, true, nil
}

func insideListItem(node ast.Node) bool {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Kind() == ast.KindListItem {
			return true
		}
	}
	return false
}

func classifySkillEntry(name string, mode fs.FileMode, isDir bool) (bool, error) {
	if mode&os.ModeSymlink != 0 {
		return false, fmt.Errorf(".agents/skills/%s: skill roots and entries cannot be symlinks", name)
	}
	return isDir, nil
}

func isWindowsLocalPath(path string) bool {
	return windowsDrivePath.MatchString(path) || strings.HasPrefix(path, `\\`)
}

func resolveInsideRoot(root, relative string) (string, error) {
	lexical := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	within, err := filepath.Rel(root, lexical)
	if err != nil {
		return "", err
	}
	if err := exactCasePath(root, within); err != nil {
		return "", fmt.Errorf("link %q: %w", relative, err)
	}
	return resolveInsideRootWith(root, relative, filepath.EvalSymlinks)
}

func resolveInsideRootWith(root, relative string, evalSymlinks func(string) (string, error)) (string, error) {
	lexical := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	within, err := filepath.Rel(root, lexical)
	if err != nil {
		return "", err
	}
	if within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return "", fmt.Errorf("link %q resolves outside the repository", relative)
	}
	resolved, err := evalSymlinks(lexical)
	if err != nil {
		return "", fmt.Errorf("link %q does not resolve: %w", relative, err)
	}
	within, err = filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return "", fmt.Errorf("link %q resolves through a symlink outside the repository", relative)
	}
	return resolved, nil
}

func exactCasePath(root, relative string) error {
	relative = filepath.Clean(relative)
	if relative == "." {
		return nil
	}
	within, err := filepath.Rel(root, filepath.Join(root, relative))
	if err != nil {
		return err
	}
	if within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return fmt.Errorf("path %q resolves outside the repository", relative)
	}
	current := root
	parts := strings.Split(within, string(filepath.Separator))
	for index, part := range parts {
		entries, err := os.ReadDir(current)
		if err != nil {
			return err
		}
		found := false
		caseMismatch := ""
		for _, entry := range entries {
			if entry.Name() == part {
				found = true
				break
			}
			if caseMismatch == "" && strings.EqualFold(entry.Name(), part) {
				caseMismatch = entry.Name()
			}
		}
		path := filepath.ToSlash(filepath.Join(parts[:index+1]...))
		if !found {
			if caseMismatch != "" {
				return fmt.Errorf("path %q uses wrong case %q; expected %q", path, part, caseMismatch)
			}
			return fmt.Errorf("path %q does not exist", path)
		}
		current = filepath.Join(current, part)
	}
	return nil
}
