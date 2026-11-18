package check

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
				writeTestFile(t, root, "README.md", "# Root\nText[^1] indexes grid[row][col].\n\n[^1]: explanatory note\n")
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
			name: "GitHub preserves punctuation-created double hyphen",
			setup: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/templates/assignment.md", "# Assignment: <role> / <task-id>\n")
				writeTestFile(t, root, "README.md", "# Root\n[assignment](docs/templates/assignment.md#assignment-role--task-id)\n")
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
			name: "non-GitHub collapsed assignment anchor",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "docs/templates/assignment.md", "# Assignment: <role> / <task-id>\n")
				writeTestFile(t, root, "README.md", "# Root\n[assignment](docs/templates/assignment.md#assignment-role-task-id)\n")
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
			name: "broken reference link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "README.md", "# Root\n[missing][guide]\n\n[guide]: missing.md\n")
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

func TestResolveInsideRootRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.md")
	_, err := resolveInsideRootWith(root, "linked.md", func(path string) (string, error) {
		if path != filepath.Join(root, "linked.md") {
			t.Fatalf("resolved wrong input: %s", path)
		}
		return outside, nil
	})
	if err == nil {
		t.Fatal("accepted evaluated link outside the repository")
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
