package check

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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
	unusedFootnotesKey     = parser.NewContextKey()
	workflowMarkdownParser = goldmark.New(
		goldmark.WithExtensions(extension.Footnote),
		goldmark.WithParserOptions(parser.WithASTTransformers(
			util.Prioritized(collectUnusedFootnotes{}, 998),
		)),
	).Parser()
	bareRouteToken   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	windowsDrivePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

type markdownDocument struct {
	source          []byte
	root            ast.Node
	references      []parser.Reference
	unusedFootnotes []*extensionast.Footnote
}

type workflowPaths struct {
	root       *os.Root
	sourceRoot string
}

type workflowLinkContext struct {
	paths       workflowPaths
	documents   map[string]markdownDocument
	anchors     map[string]map[string]bool
	resolutions map[string]pathResolution
}

type pathResolution struct {
	path string
	err  error
}

func Workflow(root string) (int, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return 0, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return 0, err
	}
	rootFS, err := os.OpenRoot(resolved)
	if err != nil {
		return 0, err
	}
	defer rootFS.Close()
	paths := workflowPaths{root: rootFS, sourceRoot: absolute}
	files, skills, err := workflowInventory(paths)
	if err != nil {
		return 0, err
	}
	documents := make(map[string]markdownDocument, len(files))
	for _, file := range files {
		data, err := paths.readFile(file)
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
		paths:       paths,
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

func workflowInventory(paths workflowPaths) ([]string, map[string]string, error) {
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

	entries, err := paths.readDir(".agents/skills")
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
		data, err := paths.readFile(relative)
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

	templates, err := paths.readDir("docs/templates")
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

func (paths workflowPaths) readDir(relative string) ([]fs.DirEntry, error) {
	resolved, err := paths.resolve(relative, false)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	entries, err := fs.ReadDir(paths.root.FS(), resolved)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	return entries, nil
}

func (paths workflowPaths) readFile(relative string) ([]byte, error) {
	resolved, err := paths.resolve(relative, false)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	info, err := paths.root.Lstat(resolved)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: workflow input is not a regular file", relative)
	}
	data, err := paths.root.ReadFile(resolved)
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

type collectUnusedFootnotes struct{}

func (collectUnusedFootnotes) Transform(document *ast.Document, _ text.Reader, context parser.Context) {
	// The footnote renderer's transform removes these definitions from the AST.
	var unused []*extensionast.Footnote
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if note, ok := node.(*extensionast.Footnote); entering && ok && note.Index < 0 {
			unused = append(unused, note)
		}
		return ast.WalkContinue, nil
	})
	context.Set(unusedFootnotesKey, unused)
}

func parseMarkdownDocument(source []byte) markdownDocument {
	context := parser.NewContext()
	root := workflowMarkdownParser.Parse(text.NewReader(source), parser.WithContext(context))
	return markdownDocument{
		source:          source,
		root:            root,
		references:      context.References(),
		unusedFootnotes: context.Get(unusedFootnotesKey).([]*extensionast.Footnote),
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
			name, named, err := routeLinkName(link, document.source)
			if err != nil {
				return ast.WalkStop, err
			}
			target, err := parseLocalMarkdownTarget("AGENTS.md", string(link.Destination))
			if err != nil {
				return ast.WalkStop, err
			}
			var targetPath string
			if target != nil {
				targetPath = markdownTargetPath("AGENTS.md", target.Path)
			}
			if !named {
				parts := strings.Split(targetPath, "/")
				if len(parts) == 4 && parts[0] == ".agents" && parts[1] == "skills" && parts[3] == "SKILL.md" {
					return ast.WalkStop, fmt.Errorf("AGENTS.md: skill entrypoint %q must have a code label matching the skill name", targetPath)
				}
				return ast.WalkContinue, nil
			}
			relative, exists := skills[name]
			if !exists {
				return ast.WalkStop, fmt.Errorf("AGENTS.md: routed skill %q has no .agents/skills/%s/SKILL.md", name, name)
			}
			if target == nil || targetPath != relative || target.ForceQuery || target.RawQuery != "" || target.Fragment != "" {
				return ast.WalkStop, fmt.Errorf("AGENTS.md: routed skill %q must link to %s", name, relative)
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
		if err := validateMarkdownTarget(context, relative, label, string(reference.Destination())); err != nil {
			return err
		}
	}
	validate := func(node ast.Node, entering bool) (ast.WalkStatus, error) {
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
	}
	for _, definition := range document.unusedFootnotes {
		if err := ast.Walk(definition, validate); err != nil {
			return err
		}
	}
	return ast.Walk(document.root, validate)
}

func parseLocalMarkdownTarget(relative, target string) (*url.URL, error) {
	if isWindowsLocalPath(target) {
		return nil, fmt.Errorf("%s: link %q is a Windows local absolute path outside the repository", relative, target)
	}
	normalized := string(util.URLEscape([]byte(target), true))
	if isWindowsLocalPath(normalized) {
		return nil, fmt.Errorf("%s: link %q is a Windows local absolute path outside the repository", relative, target)
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid Markdown link %q: %w", relative, target, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(normalized, "//") {
		return nil, nil
	}
	if filepath.IsAbs(filepath.FromSlash(parsed.Path)) {
		return nil, fmt.Errorf("%s: absolute repository link %q is not allowed", relative, target)
	}
	return parsed, nil
}

func markdownTargetPath(relative, destination string) string {
	if destination == "" {
		return relative
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(relative)), filepath.FromSlash(destination)))
}

func validateMarkdownTarget(context *workflowLinkContext, relative, label, target string) error {
	parsed, err := parseLocalMarkdownTarget(relative, target)
	if err != nil || parsed == nil {
		return err
	}
	targetPath := markdownTargetPath(relative, parsed.Path)
	resolution, ok := context.resolutions[targetPath]
	if !ok {
		resolution.path, resolution.err = context.paths.resolve(targetPath, true)
		context.resolutions[targetPath] = resolution
	}
	if resolution.err != nil {
		return fmt.Errorf("%s: link %q: %w", relative, target, resolution.err)
	}
	info, err := context.paths.root.Stat(resolution.path)
	if err != nil {
		return fmt.Errorf("%s: reference %q link %q does not resolve: %w", relative, label, target, err)
	}
	if parsed.Fragment == "" {
		return nil
	}
	if info.IsDir() || !strings.EqualFold(filepath.Ext(resolution.path), ".md") {
		return fmt.Errorf("%s: link %q has a fragment but target is not Markdown", relative, target)
	}
	anchors, err := context.markdownAnchors(resolution.path)
	if err != nil {
		return fmt.Errorf("%s: read fragment target %q: %w", relative, target, err)
	}
	fragment := strings.ToLower(parsed.Fragment)
	if !anchors[fragment] {
		return fmt.Errorf("%s: link %q names missing Markdown heading %q", relative, target, parsed.Fragment)
	}
	return nil
}

func (context *workflowLinkContext) markdownAnchors(resolved string) (map[string]bool, error) {
	if anchors, ok := context.anchors[resolved]; ok {
		return anchors, nil
	}
	document, ok := context.documents[resolved]
	if !ok {
		data, err := context.paths.root.ReadFile(resolved)
		if err != nil {
			return nil, err
		}
		document = parseMarkdownDocument(data)
		context.documents[resolved] = document
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

func (paths workflowPaths) resolve(relative string, allowSymlinks bool) (string, error) {
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return "", errors.New("path resolves outside the repository")
	}
	if !fs.ValidPath(relative) {
		return "", fmt.Errorf("invalid repository path %q", relative)
	}
	const maxSymlinks = 255
	var directories []string
	symlinks := 0
	pending := strings.Split(relative, "/")
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(directories) == 0 {
				return "", errors.New("path resolves through a symlink outside the repository")
			}
			directories = directories[:len(directories)-1]
			continue
		}
		parent := path.Join(".", strings.Join(directories, "/"))
		name := path.Join(parent, part)
		entry, err := exactWorkflowEntry(paths.root.FS(), parent, part)
		if err != nil {
			return "", err
		}
		if entry.Type()&fs.ModeSymlink == 0 {
			if len(pending) == 0 {
				return name, nil
			}
			if !entry.IsDir() {
				return "", fmt.Errorf("path %q is not a directory", name)
			}
			directories = append(directories, part)
			continue
		}
		if !allowSymlinks {
			return "", fmt.Errorf("%s: symlink is not allowed in workflow inputs", name)
		}
		symlinks++
		if symlinks > maxSymlinks {
			return "", fmt.Errorf("path %q exceeds the symbolic link limit", name)
		}
		target, err := paths.root.Readlink(name)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			var ok bool
			target, ok = paths.relativeLinkTarget(target)
			if !ok {
				return "", fmt.Errorf("path %q resolves through a symlink outside the repository", name)
			}
			directories = directories[:0]
		} else if filepath.VolumeName(target) != "" || strings.HasPrefix(filepath.ToSlash(target), "/") {
			return "", fmt.Errorf("path %q resolves through a symlink outside the repository", name)
		}
		pending = append(strings.Split(filepath.ToSlash(target), "/"), pending...)
	}
	return path.Join(".", strings.Join(directories, "/")), nil
}

func (paths workflowPaths) relativeLinkTarget(absolute string) (string, bool) {
	volume := filepath.VolumeName(absolute)
	target := strings.Split(filepath.ToSlash(absolute[len(volume):]), "/")
	// Match the known root without cleaning any unresolved suffix components.
roots:
	for _, root := range []string{paths.root.Name(), paths.sourceRoot} {
		rootVolume := filepath.VolumeName(root)
		if !strings.EqualFold(volume, rootVolume) {
			continue
		}
		remaining := target
		for _, part := range strings.Split(filepath.ToSlash(root[len(rootVolume):]), "/") {
			if part == "" {
				continue
			}
			for len(remaining) > 0 && (remaining[0] == "" || remaining[0] == ".") {
				remaining = remaining[1:]
			}
			if len(remaining) == 0 || (remaining[0] != part && (filepath.Separator != '\\' || !strings.EqualFold(remaining[0], part))) {
				continue roots
			}
			remaining = remaining[1:]
		}
		return strings.Join(remaining, "/"), true
	}
	return "", false
}

func exactWorkflowEntry(root fs.FS, parent, part string) (fs.DirEntry, error) {
	entries, err := fs.ReadDir(root, parent)
	if err != nil {
		return nil, err
	}
	caseMismatch := ""
	for _, entry := range entries {
		if entry.Name() == part {
			return entry, nil
		}
		if caseMismatch == "" && strings.EqualFold(entry.Name(), part) {
			caseMismatch = entry.Name()
		}
	}
	name := path.Join(parent, part)
	if caseMismatch != "" {
		return nil, fmt.Errorf("path %q uses wrong case %q; expected %q", name, part, caseMismatch)
	}
	return nil, fmt.Errorf("path %q does not exist", name)
}
