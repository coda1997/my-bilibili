package check

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
)

func TestFormattingIsReadOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	original := []byte("package sample\nfunc Value()int{return 1}\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Formatting(root); err == nil {
		t.Fatal("accepted unformatted source")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("formatting check changed the source")
	}
	if err := os.WriteFile(path, []byte("package sample\n\nfunc Value() int { return 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent", "ignored.go"), []byte("not Go source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Formatting(root); err != nil {
		t.Fatal(err)
	}
}

func TestRunStopsBeforeCommandsOnInvalidContracts(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "specs"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Run(context.Background(), root, &output, false); err == nil {
		t.Fatal("accepted an empty spec collection")
	}
	if bytes.Contains(output.Bytes(), []byte("go vet")) {
		t.Fatal("ran later checks after contract failure")
	}
}

func TestCanceledCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if err := Run(ctx, t.TempDir(), &output, false); err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if output.Len() != 0 {
		t.Fatal("started a step after cancellation")
	}
}

func TestWorkflowAcceptsValidInventory(t *testing.T) {
	root := writeWorkflowFixture(t)
	count, err := Workflow(root)
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("workflow check covered no files")
	}
}

func TestWorkflowAcceptsValidMarkdownForms(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{
			name: "non-directory skill root entries",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/.DS_Store", "metadata")
				writeTestFile(t, root, ".agents/skills/README.md", "# Skill index\n")
			},
		},
		{
			name: "nested task-routing heading and fenced heading",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n\n## Task routing\n\n```text\n# Example heading\n```\n\n### Implementation\n\n- Sample: [`sample`](.agents/skills/sample/SKILL.md).\n")
			},
		},
		{
			name: "valid footnote and bracket identifiers",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\nText[^1] indexes grid[row][col].\n\n[^1]: note\n")
			},
		},
		{
			name: "inline-code heading anchor",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "# Run `go test`\n")
				writeTestFile(t, root, "README.md", "# Root\n[workflow](docs/workflow.md#run-go-test)\n")
			},
		},
		{
			name: "percent in encoded link path",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/name%20x.md", "# Percent\n")
				writeTestFile(t, root, "README.md", "# Root\n[percent](docs/name%2520x.md)\n")
			},
		},
		{
			name: "multiple skills and command tokens on one route line",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/other/SKILL.md", "---\nname: other\ndescription: Other skill\n---\n# Other\n")
				writeClaudeSkillMirror(t, root, "other")
				writeTestFile(t, root, "AGENTS.md", "# Agents\n\n## Task routing\n\n- Tools: run `gofmt`, then [`sample`](.agents/skills/sample/SKILL.md) and [`other`](.agents/skills/other/SKILL.md).\n")
			},
		},
		{
			name: "indented YAML block scalar",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: sample\ndescription: >2\n  Sample skill\n---\n# Sample\n")
			},
		},
		{
			name: "setext heading anchor",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "Workflow heading\n================\n")
				writeTestFile(t, root, "README.md", "# Root\n[workflow](docs/workflow.md#workflow-heading)\n")
			},
		},
		{
			name: "commented links are ignored",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n<!-- [broken](missing.md) -->\n")
			},
		},
		{
			name: "commented headings do not end task routing",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n\n## Task routing\n\n<!--\n## Not a real section\n-->\n\n- Sample: [`sample`](.agents/skills/sample/SKILL.md).\n")
			},
		},
		{
			name: "fragment comparison ignores case",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "# Mixed Case Heading\n")
				writeTestFile(t, root, "README.md", "# Root\n[workflow](docs/workflow.md#Mixed-Case-Heading)\n")
			},
		},
		{
			name: "GitHub omits HTML placeholders in assignment anchors",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/templates/assignment.md", "# Assignment: <role> / <task-id>\n")
				writeTestFile(t, root, "README.md", "# Root\n[assignment](docs/templates/assignment.md#assignment---)\n")
			},
		},
		{
			name: "heading fragments use rendered text",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "# A &amp; B\n\n# Intro <em>setup</em>\n")
				writeTestFile(t, root, "README.md", "# Root\n[entity](docs/workflow.md#a--b)\n[HTML](docs/workflow.md#intro-setup)\n")
			},
		},
		{
			name: "code span fragments preserve literal text",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "# `A &amp; B <em>`\n")
				writeTestFile(t, root, "README.md", "# Root\n[code](docs/workflow.md#a-amp-b-em)\n")
			},
		},
		{
			name: "GitHub reserves generated and literal duplicate anchors globally",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "# Foo\n\n# Foo\n\n# Foo-1\n")
				writeTestFile(t, root, "README.md", "# Root\n[first](docs/workflow.md#foo)\n[duplicate](docs/workflow.md#foo-1)\n[collision](docs/workflow.md#foo-1-1)\n")
			},
		},
		{
			name: "parentheses in local link path",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/foo(1).md", "# Parentheses\n")
				writeTestFile(t, root, "README.md", "# Root\n[fixture](docs/foo(1).md)\n")
			},
		},
		{
			name: "Markdown escapes and entities in link targets",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/foo(1).md", "# Parentheses\n")
				writeTestFile(t, root, "docs/a&b.md", "# Ampersand\n")
				writeTestFile(t, root, "README.md", "# Root\n[inline](docs/foo\\(1\\).md)\n[reference][guide]\n[entity](docs/a&amp;b.md)\n\n[guide]: docs/foo\\(1\\).md\n")
			},
		},
		{
			name: "YAML double-quoted escapes",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: \"sam\\u0070le\"\ndescription: \"Sample \\\"skill\\\"\"\n---\n# Sample\n")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeWorkflowFixture(t)
			test.setup(t, root)
			if _, err := Workflow(root); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkflowRejectsDamagedInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{
			name: "skill name mismatch",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: other\ndescription: Sample skill\n---\n# Sample\n")
			},
		},
		{
			name: "comment-only skill description",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: sample\ndescription: # absent\n---\n# Sample\n")
			},
		},
		{
			name: "empty block skill description",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: sample\ndescription: >-\n---\n# Sample\n")
			},
		},
		{
			name: "commented empty block skill description",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: sample\ndescription: >- # absent\n---\n# Sample\n")
			},
		},
		{
			name: "commented null skill description",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: sample\ndescription: null # absent\n---\n# Sample\n")
			},
		},
		{
			name: "nested frontmatter keys",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nmetadata:\n  name: sample\n  description: Sample skill\n---\n# Sample\n")
			},
		},
		{
			name: "empty indented block description",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: sample\ndescription: >2\n---\n# Sample\n")
			},
		},
		{
			name: "unclosed inline backtick cannot hide links",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\nUnclosed ` text.\n\n[missing](missing.md)\n")
			},
		},
		{
			name: "missing local link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[missing](missing.md)\n")
			},
		},
		{
			name: "broken link after mismatched fence",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n\n````markdown\n```\n````\n[missing](missing.md)\n")
			},
		},
		{
			name: "different fence character does not close block",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n\n````markdown\n~~~~\n````\n[missing](missing.md)\n")
			},
		},
		{
			name: "case-mismatched local link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[workflow](docs/Workflow.md)\n")
			},
		},
		{
			name: "windows drive absolute link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[outside](C:/outside.md)\n")
			},
		},
		{
			name: "windows drive backslash link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[outside](C:\\outside.md)\n")
			},
		},
		{
			name: "windows UNC link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[outside](\\\\server\\share\\outside.md)\n")
			},
		},
		{
			name: "route after same-level heading is outside section",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n\n## Task routing\n\nNo route here.\n\n## Next section\n\n- Sample: [`sample`](.agents/skills/sample/SKILL.md).\n")
			},
		},
		{
			name: "missing heading fragment",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[missing](docs/workflow.md#missing-heading)\n")
			},
		},
		{
			name: "assignment anchor includes HTML placeholder names",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/templates/assignment.md", "# Assignment: <role> / <task-id>\n")
				writeTestFile(t, root, "README.md", "# Root\n[assignment](docs/templates/assignment.md#assignment-role--task-id)\n")
			},
		},
		{
			name: "heading fragment includes entity name",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "# A &amp; B\n")
				writeTestFile(t, root, "README.md", "# Root\n[entity](docs/workflow.md#a-amp-b)\n")
			},
		},
		{
			name: "heading fragment includes HTML tag names",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/workflow.md", "# Intro <em>setup</em>\n")
				writeTestFile(t, root, "README.md", "# Root\n[HTML](docs/workflow.md#intro-emsetupem)\n")
			},
		},
		{
			name: "missing skill route",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n## Task routing\n\n- Planning: no skill route.\n")
			},
		},
		{
			name: "unknown skill route",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n## Task routing\n\n- Planning: [`missing-skill`](.agents/skills/missing-skill/SKILL.md).\n")
			},
		},
		{
			name: "unknown skill route with existing target",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n## Task routing\n\n- Sample: [`sample`](.agents/skills/sample/SKILL.md).\n- Planning: [`missing-skill`](docs/workflow.md).\n")
			},
		},
		{
			name: "skill in comment is not a route",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n## Task routing\n\n<!-- Planning: [`sample`](.agents/skills/sample/SKILL.md). -->\n")
			},
		},
		{
			name: "skill in fenced example is not a route",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n## Task routing\n\n```text\n- Planning: [`sample`](.agents/skills/sample/SKILL.md).\n```\n")
			},
		},
		{
			name: "duplicate skill route",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n## Task routing\n\n- Planning: [`sample`](.agents/skills/sample/SKILL.md).\n- Repair: [`sample`](.agents/skills/sample/SKILL.md).\n")
			},
		},
		{
			name: "duplicate skill route with different label formatting",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n## Task routing\n\n- Planning: [`sample`](.agents/skills/sample/SKILL.md).\n- Repair: [sample](./.agents/skills/sample/SKILL.md).\n")
			},
		},
		{
			name: "broken reference link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[missing][guide]\n\n[guide]: missing.md\n")
			},
		},
		{
			name: "broken unused reference link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n\n[guide]: missing.md\n")
			},
		},
		{
			name: "broken link in unused footnote",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n\n[^1]: [missing](missing.md)\n")
			},
		},
		{
			name: "outside root link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[outside](../outside.md)\n")
				writeTestFile(t, filepath.Dir(root), "outside.md", "# Outside\n")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeWorkflowFixture(t)
			test.mutate(t, root)
			if _, err := Workflow(root); err == nil {
				t.Fatal("accepted damaged workflow input")
			}
		})
	}
}

func TestWorkflowRejectsSkillSymlinkEntry(t *testing.T) {
	root := writeWorkflowFixture(t)
	target := filepath.Join(root, "README.md")
	link := filepath.Join(root, ".agents", "skills", "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if _, err := Workflow(root); err == nil {
		t.Fatal("accepted symlinked skill entry")
	}
}

func TestWorkflowRejectsWrongCaseInventoryFile(t *testing.T) {
	root := writeWorkflowFixture(t)
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "readme.md", "# Root\n")
	if _, err := Workflow(root); err == nil || !strings.Contains(err.Error(), `path "README.md" uses wrong case`) || !strings.Contains(err.Error(), `"readme.md"`) {
		t.Fatalf("want exact-case inventory diagnostic, got %v", err)
	}
}

func TestWorkflowRejectsSymlinkEscapeBeforeCheckingSuffix(t *testing.T) {
	root := writeWorkflowFixture(t)
	outside := t.TempDir()
	writeTestFile(t, outside, "Outside.md", "# Outside\n")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	target := filepath.FromSlash("linked/../README.md")
	if filepath.Separator != '\\' {
		target = root + string(filepath.Separator) + target
	}
	through := filepath.Join(root, "through")
	if err := os.Symlink(target, through); err != nil {
		t.Fatal(err)
	}
	if stored, err := os.Readlink(through); err != nil || stored != target {
		t.Fatalf("fixture must preserve the unresolved symlink target: got %q, err=%v", stored, err)
	}
	for _, destination := range []string{"linked/outside.md", "through"} {
		writeTestFile(t, root, "README.md", "# Root\n[outside]("+destination+")\n")
		if _, err := Workflow(root); err == nil || !strings.Contains(err.Error(), "symlink outside the repository") || strings.Contains(err.Error(), "Outside.md") {
			t.Errorf("%s: want boundary diagnostic without outside path details, got %v", destination, err)
		}
	}
}

func TestWorkflowAcceptsInRootSymlinks(t *testing.T) {
	root := writeWorkflowFixture(t)
	if err := os.Symlink("docs", filepath.Join(root, "relative")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "docs"), filepath.Join(root, "absolute")); err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)
	target := filepath.Dir(root) + sep + "." + sep + filepath.Base(root) + sep + "docs"
	if err := os.Symlink(target, filepath.Join(root, "dot-absolute")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "README.md", "# Root\n[relative](relative/workflow.md#workflow)\n[absolute](absolute/workflow.md#workflow)\n[dot absolute](dot-absolute/workflow.md#workflow)\n")
	if _, err := Workflow(root); err != nil {
		t.Fatal(err)
	}
}

func TestClassifySkillEntryRejectsSymlinkOnEveryPlatform(t *testing.T) {
	if _, err := classifySkillEntry("linked", fs.ModeSymlink, false); err == nil {
		t.Fatal("accepted symlink skill entry")
	}
}

func TestWalkNodeTextPropagatesVisitorError(t *testing.T) {
	document := parseMarkdownDocument([]byte("# Heading\n"))
	want := errors.New("visitor failed")
	err := walkNodeText(document.root, document.source, func(ast.Node, []byte) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func TestWorkflowRejectsLinkCycles(t *testing.T) {
	root := writeWorkflowFixture(t)
	if err := os.Symlink("loop", filepath.Join(root, "loop")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	writeTestFile(t, root, "README.md", "# Root\n[loop](loop)\n")
	if _, err := Workflow(root); err == nil || !strings.Contains(err.Error(), "symbolic link limit") {
		t.Fatalf("want bounded symlink resolution, got %v", err)
	}
}

func TestRunStopsBeforeCommandsOnInvalidWorkflow(t *testing.T) {
	root := writeWorkflowFixture(t)
	writeTestFile(t, root, "README.md", "# Root\n[missing](missing.md)\n")
	writeTestFile(t, root, "specs/valid/spec.md", "# Valid\n\n### TEST-AC-01\n\nFixture acceptance.\n")
	writeTestFile(t, root, "specs/valid/tasks.json", `{
  "schema_version": 1,
  "spec_id": "valid",
  "tasks": [{
    "id": "TEST-001",
    "title": "Fixture",
    "depends_on": [],
    "write_paths": ["README.md"],
    "acceptance": ["TEST-AC-01"],
    "checks": ["go test ./..."]
  }]
}`)
	var output bytes.Buffer
	if err := Run(context.Background(), root, &output, false); err == nil {
		t.Fatal("accepted invalid workflow input")
	}
	if !strings.Contains(output.String(), "agent workflow integrity") {
		t.Fatal("canonical check did not execute the workflow step")
	}
	if strings.Contains(output.String(), "Go formatting") {
		t.Fatal("canonical check continued after workflow failure")
	}
}

func writeWorkflowFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "# Root\n")
	writeTestFile(t, root, "AGENTS.md", "# Agents\n\n## Task routing\n\n- Sample: [`sample`](.agents/skills/sample/SKILL.md).\n")
	writeTestFile(t, root, "STATUS.md", "# Status\n")
	writeTestFile(t, root, ".github/copilot-instructions.md", "# Copilot\n")
	writeTestFile(t, root, ".github/pull_request_template.md", "# Pull request\n")
	writeTestFile(t, root, ".agents/skills/sample/SKILL.md", "---\nname: sample\ndescription: Sample skill\n---\n# Sample\n")
	writeClaudeSkillMirror(t, root, "sample")
	writeTestFile(t, root, "docs/templates/spec.md", "# Spec template\n")
	for _, relative := range []string{
		"agent-convergence.md",
		"architecture.md",
		"development.md",
		"pull-requests.md",
		"testing.md",
		"workflow.md",
	} {
		writeTestFile(t, root, filepath.Join("docs", relative), "# Workflow\n")
	}
	return root
}

func writeTestFile(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeClaudeSkillMirror(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, ".claude", "skills")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := "../../.agents/skills/" + name
	link := filepath.Join(dir, name)
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			if writeErr := os.WriteFile(link, []byte(target), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			return
		}
		t.Fatal(err)
	}
}

func writeSpecFixture(t *testing.T, root, specID, acID string) {
	t.Helper()
	writeTestFile(t, root, "specs/"+specID+"/spec.md", "# "+specID+"\n\n### "+acID+"\n\nObservable criterion.\n")
	writeTestFile(t, root, "specs/"+specID+"/tasks.json", `{
  "schema_version": 1,
  "spec_id": "`+specID+`",
  "tasks": [{
    "id": "T1",
    "title": "Fixture task",
    "depends_on": [],
    "write_paths": ["README.md"],
    "acceptance": ["`+acID+`"],
    "checks": ["go test ./..."]
  }]
}`)
}

func runClaudeSkillMirrorCheck(t *testing.T, root string, isWindows bool) error {
	t.Helper()
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		t.Fatal(err)
	}
	rootFS, err := os.OpenRoot(resolved)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFS.Close()
	paths := workflowPaths{root: rootFS, sourceRoot: absolute}
	_, skills, err := workflowInventory(paths)
	if err != nil {
		t.Fatal(err)
	}
	return validateClaudeSkillMirrors(paths, skills, isWindows)
}

func TestWorkflowClaudeSkillMirrors(t *testing.T) {
	t.Run("accepts regular DS_Store in claude skills", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		writeTestFile(t, root, ".claude/skills/.DS_Store", "metadata")
		if _, err := Workflow(root); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("rejects missing claude skills directory", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		if err := os.RemoveAll(filepath.Join(root, ".claude")); err != nil {
			t.Fatal(err)
		}
		if _, err := Workflow(root); err == nil {
			t.Fatal("accepted missing .claude/skills directory")
		}
	})

	t.Run("rejects symlinked claude skills directory", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		if err := os.RemoveAll(filepath.Join(root, ".claude", "skills")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../.agents/skills", filepath.Join(root, ".claude", "skills")); err != nil {
			t.Skipf("symlink fixture unavailable: %v", err)
		}
		if _, err := Workflow(root); err == nil {
			t.Fatal("accepted symlinked .claude/skills root")
		}
	})

	t.Run("rejects missing mirror for canonical skill", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		if err := os.Remove(filepath.Join(root, ".claude", "skills", "sample")); err != nil {
			t.Fatal(err)
		}
		if _, err := Workflow(root); err == nil || !strings.Contains(err.Error(), "missing Claude skill mirror") {
			t.Fatalf("want missing mirror diagnostic, got %v", err)
		}
	})

	t.Run("rejects unexpected stale entry", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		writeClaudeSkillMirror(t, root, "stale")
		if _, err := Workflow(root); err == nil || !strings.Contains(err.Error(), "unexpected entry") {
			t.Fatalf("want unexpected entry diagnostic, got %v", err)
		}
	})

	t.Run("rejects directory and symlink DS_Store", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		if err := os.Mkdir(filepath.Join(root, ".claude", "skills", ".DS_Store"), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := Workflow(root); err == nil || !strings.Contains(err.Error(), ".DS_Store must be a regular file") {
			t.Fatalf("want regular-file DS_Store diagnostic, got %v", err)
		}
		if err := os.Remove(filepath.Join(root, ".claude", "skills", ".DS_Store")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../README.md", filepath.Join(root, ".claude", "skills", ".DS_Store")); err != nil {
			t.Skipf("symlink fixture unavailable: %v", err)
		}
		if _, err := Workflow(root); err == nil || !strings.Contains(err.Error(), ".DS_Store must be a regular file") {
			t.Fatalf("want regular-file DS_Store diagnostic for symlink, got %v", err)
		}
	})

	t.Run("rejects mistargeted symlink and directory copy", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		link := filepath.Join(root, ".claude", "skills", "sample")
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(link, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := runClaudeSkillMirrorCheck(t, root, false); err == nil {
			t.Fatal("accepted directory copy on Unix")
		}
		if err := runClaudeSkillMirrorCheck(t, root, true); err == nil {
			t.Fatal("accepted directory copy on Windows")
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../.agents/skills/other", link); err != nil {
			t.Skipf("symlink fixture unavailable: %v", err)
		}
		if err := runClaudeSkillMirrorCheck(t, root, false); err == nil || !strings.Contains(err.Error(), "symlink target") {
			t.Fatalf("want wrong symlink target diagnostic on Unix, got %v", err)
		}
		if err := runClaudeSkillMirrorCheck(t, root, true); err == nil || !strings.Contains(err.Error(), "symlink target") {
			t.Fatalf("want wrong symlink target diagnostic on Windows, got %v", err)
		}
	})

	t.Run("windows pointer and backslash modes vs unix strictness", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		entry := filepath.Join(root, ".claude", "skills", "sample")
		if err := os.Remove(entry); err != nil {
			t.Fatal(err)
		}
		// Exact plain-file Git core.symlinks=false pointer: valid on Windows, rejected on Unix.
		writeTestFile(t, root, ".claude/skills/sample", "../../.agents/skills/sample")
		if err := runClaudeSkillMirrorCheck(t, root, true); err != nil {
			t.Fatalf("rejected valid Windows regular-file pointer: %v", err)
		}
		if err := runClaudeSkillMirrorCheck(t, root, false); err == nil {
			t.Fatal("accepted regular-file pointer on Unix")
		}
		// Trailing newline must be rejected even on Windows.
		writeTestFile(t, root, ".claude/skills/sample", "../../.agents/skills/sample\n")
		if err := runClaudeSkillMirrorCheck(t, root, true); err == nil {
			t.Fatal("accepted Windows pointer with trailing newline")
		}
		// Backslash symlink target: valid on Windows, rejected on Unix.
		if err := os.Remove(entry); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(`..\..\.agents\skills\sample`, entry); err != nil {
			t.Skipf("symlink fixture unavailable: %v", err)
		}
		if err := runClaudeSkillMirrorCheck(t, root, true); err != nil {
			t.Fatalf("rejected Windows backslash symlink target: %v", err)
		}
		if err := runClaudeSkillMirrorCheck(t, root, false); err == nil {
			t.Fatal("accepted backslash symlink target on Unix")
		}
	})
}

func TestSpecsCrossSpecUniqueness(t *testing.T) {
	t.Run("accepts grandfathered historical pairs and non-numeric fixture", func(t *testing.T) {
		root := t.TempDir()
		writeSpecFixture(t, root, "valid", "VAL-AC-01")
		writeSpecFixture(t, root, "0004-client-p0", "P0-AC-01")
		writeSpecFixture(t, root, "0004-workflow-clarifications", "WFC-AC-01")
		writeSpecFixture(t, root, "0015-m2-fixture-preview", "M2-AC-01")
		writeSpecFixture(t, root, "0016-m2-policy-continuation", "M2-AC-01")
		writeSpecFixture(t, root, "0025-agent-workflow-integrity", "AWF-AC-01")
		writeSpecFixture(t, root, "0025-p0-async-source", "ASY-AC-01")
		if err := Specs(root); err != nil {
			t.Fatalf("rejected valid grandfathered specs: %v", err)
		}
	})

	cases := []struct {
		name  string
		setup func(*testing.T, string)
		want  string
	}{
		{
			name: "new duplicate numeric prefix",
			setup: func(t *testing.T, root string) {
				writeSpecFixture(t, root, "0031-alpha", "ALP-AC-01")
				writeSpecFixture(t, root, "0031-beta", "BET-AC-01")
			},
			want: `duplicate spec numeric prefix "0031" shared by: 0031-alpha, 0031-beta`,
		},
		{
			name: "unauthorized pair under grandfathered numeric prefix 0004",
			setup: func(t *testing.T, root string) {
				writeSpecFixture(t, root, "0004-client-p0", "P0-AC-01")
				writeSpecFixture(t, root, "0004-new", "NEW-AC-01")
			},
			want: `duplicate spec numeric prefix "0004" shared by: 0004-client-p0, 0004-new`,
		},
		{
			name: "third spec under grandfathered numeric prefix 0025",
			setup: func(t *testing.T, root string) {
				writeSpecFixture(t, root, "0025-agent-workflow-integrity", "AWF-AC-01")
				writeSpecFixture(t, root, "0025-p0-async-source", "ASY-AC-01")
				writeSpecFixture(t, root, "0025-extra", "EXT-AC-01")
			},
			want: `duplicate spec numeric prefix "0025" shared by: 0025-agent-workflow-integrity, 0025-extra, 0025-p0-async-source`,
		},
		{
			name: "new duplicate acceptance prefix",
			setup: func(t *testing.T, root string) {
				writeSpecFixture(t, root, "0031-alpha", "DUP-AC-01")
				writeSpecFixture(t, root, "0032-beta", "DUP-AC-02")
			},
			want: `duplicate acceptance prefix "DUP" shared by: 0031-alpha, 0032-beta`,
		},
		{
			name: "unauthorized pair under grandfathered acceptance prefix M2",
			setup: func(t *testing.T, root string) {
				writeSpecFixture(t, root, "0015-m2-fixture-preview", "M2-AC-01")
				writeSpecFixture(t, root, "0031-other", "M2-AC-02")
			},
			want: `duplicate acceptance prefix "M2" shared by: 0015-m2-fixture-preview, 0031-other`,
		},
		{
			name: "third spec under grandfathered acceptance prefix M2",
			setup: func(t *testing.T, root string) {
				writeSpecFixture(t, root, "0015-m2-fixture-preview", "M2-AC-01")
				writeSpecFixture(t, root, "0016-m2-policy-continuation", "M2-AC-01")
				writeSpecFixture(t, root, "0031-third", "M2-AC-02")
			},
			want: `duplicate acceptance prefix "M2" shared by: 0015-m2-fixture-preview, 0016-m2-policy-continuation, 0031-third`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			err := Specs(root)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want substring %q", err, tc.want)
			}
		})
	}

	t.Run("Run fails fast before go vet on duplicate spec prefix", func(t *testing.T) {
		root := writeWorkflowFixture(t)
		writeSpecFixture(t, root, "0031-alpha", "ALP-AC-01")
		writeSpecFixture(t, root, "0031-beta", "BET-AC-01")
		var output bytes.Buffer
		err := Run(context.Background(), root, &output, false)
		if err == nil || !strings.Contains(err.Error(), `duplicate spec numeric prefix "0031"`) {
			t.Fatalf("want duplicate prefix error, got %v", err)
		}
		if strings.Contains(output.String(), "agent workflow integrity") || strings.Contains(output.String(), "go vet") {
			t.Fatalf("Run continued after spec contract collision:\n%s", output.String())
		}
	})
}
