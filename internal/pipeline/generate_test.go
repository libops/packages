package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/libops/packages/internal/catalog"
	"github.com/libops/packages/internal/config"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestWorkflowBuildDoesNotWaitForParentTest(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	parent, child := "alpine-3-24-php-8-4", "alpine-3-24-nginx-1-30"
	p := c[child]
	p.Dependencies = []string{parent}
	c[child] = p
	workflow, err := Workflow("../..", c)
	if err != nil {
		t.Fatal(err)
	}
	job := strings.Split(strings.Split(workflow, "  build-"+child+":")[1], "  test-"+child+":")[0]
	if !strings.Contains(job, "build-"+parent) || strings.Contains(job, "test-"+parent) {
		t.Fatal(job)
	}
}
func TestRequiredGate(t *testing.T) {
	data, err := os.ReadFile("../../ci/push.yml.template")
	if err != nil {
		t.Fatal(err)
	}
	gate := strings.Split(strings.Split(string(data), "      - name: Check every selected build and test\n        run: |\n")[1], "\n  docs-build:")[0]
	for _, tc := range []struct {
		build, test string
		pass        bool
	}{{"success", "success", true}, {"failure", "skipped", false}, {"success", "failure", false}, {"skipped", "skipped", false}, {"success", "cancelled", false}, {"success", "skipped", false}} {
		for _, fork := range []bool{false, true} {
			expected := tc.pass || fork && tc.build == "success" && tc.test == "skipped"
			cmd := exec.Command("bash", "-e", "-c", gate)
			cmd.Env = append(os.Environ(), "FORK="+strconv.FormatBool(fork), `TARGETS=["example"]`, `RESULTS={"install-check":{"result":"success"},"plan":{"result":"success"},"build-example":{"result":"`+tc.build+`"},"test-example":{"result":"`+tc.test+`"}}`)
			if err := cmd.Run(); (err == nil) != expected {
				t.Fatalf("build %s test %s: %v", tc.build, tc.test, err)
			}
		}
	}
}

func TestConcurrencySeparatesPRsAndMainPushes(t *testing.T) {
	for _, file := range []string{"../../ci/push.yml.template"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "github.event_name == 'pull_request' && github.event.pull_request.number || github.run_id") || !strings.Contains(text, "cancel-in-progress: ${{ github.event_name == 'pull_request' }}") {
			t.Fatalf("%s: missing per-PR cancellation or independent main run", file)
		}
	}
}

func TestRenovateExtractsActualRecipesAndStaysWithinRuntimeLine(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	config := Renovate(c)
	for _, entry := range config["customManagers"].([]any) {
		manager := entry.(map[string]any)
		name := strings.TrimSuffix(manager["depNameTemplate"].(string), "-source")
		p, ok := c[name]
		if !ok {
			continue
		}
		data, err := os.ReadFile("../../packages/" + p.Target + "/APKBUILD")
		if err != nil {
			t.Fatal(err)
		}
		pattern := regexp.MustCompile(manager["matchStrings"].([]string)[0])
		match := pattern.FindStringSubmatch(string(data))
		if match == nil {
			t.Fatalf("%s: cannot extract current source", name)
		}
		version := match[pattern.SubexpIndex("currentValue")]
		if p.Update.Tag && p.Update.Extract != "" {
			extract := regexp.MustCompile(p.Update.Extract)
			m := extract.FindStringSubmatch(version)
			if m == nil {
				t.Fatalf("%s: source tag extraction failed for %s", name, version)
			}
			version = m[extract.SubexpIndex("version")]
		}
		allowed := regexp.MustCompile(strings.Trim(p.Update.Allowed, "/"))
		if !allowed.MatchString(version) {
			t.Errorf("%s: current version %s rejected", name, version)
		}
		if allowed.MatchString(version + "-rc1") {
			t.Errorf("%s: prerelease accepted", name)
		}
		switch p.Family {
		case "php", "python", "ruby", "go":
			parts := strings.Split(p.Line, ".")
			minor, err := strconv.Atoi(parts[1])
			if err != nil {
				t.Fatal(err)
			}
			next := fmt.Sprintf("%s.%d.0", parts[0], minor+1)
			if allowed.MatchString(next) {
				t.Errorf("%s: next minor %s accepted", name, next)
			}
		}
	}
}

func TestWorkflowCredentialsAreScoped(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := Workflow("../..", c)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"permissions: {}", "persist-credentials: false", "REGISTRY_USER:", "REGISTRY_PASSWORD:"} {
		if !strings.Contains(workflow, required) {
			t.Fatal("missing", required)
		}
	}
	if strings.Contains(workflow, "secrets: inherit") || strings.Contains(workflow, "GH_APP_PRIV_KEY") {
		t.Fatal("package DAG exposes unrelated secrets")
	}
	for _, rule := range Renovate(c)["packageRules"].([]any) {
		r := rule.(map[string]any)
		if r["automerge"] == true {
			t.Fatal("unverified source updates may not auto-merge")
		}
	}
}

func TestToolRulesFollowDeclaredPins(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/Dockerfile", []byte("ARG GO_IMAGE=golang:1.99.7-alpine9.8@sha256:"+strings.Repeat("a", 64)+"\n"), 0644)
	os.WriteFile(root+"/Dockerfile.docs", []byte("FROM python:3.88.1-slim-example@sha256:"+strings.Repeat("b", 64)+"\n"), 0644)
	rules, err := ToolRules(root)
	if err != nil {
		t.Fatal(err)
	}
	for i, allowed := range []struct{ good, bad string }{{"1.99.8-alpine9.8", "1.100.0-alpine9.8"}, {"3.88.2-slim-example", "3.89.0-slim-example"}} {
		pattern := regexp.MustCompile(strings.Trim(rules[i].(map[string]any)["allowedVersions"].(string), "/"))
		if !pattern.MatchString(allowed.good) || pattern.MatchString(allowed.bad) {
			t.Fatal(pattern)
		}
	}
}

func TestSimpleUpdaterAllowlist(t *testing.T) {
	data, err := os.ReadFile("../../ci/renovate-allowed-commands.json")
	if err != nil {
		t.Fatal(err)
	}
	var patterns []string
	if err = json.Unmarshal(data, &patterns); err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 1 {
		t.Fatal(patterns)
	}
	pattern := regexp.MustCompile(patterns[0])
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range c.Names() {
		if !pattern.MatchString("make update TARGET=" + name) {
			t.Fatal(name)
		}
	}
	if !pattern.MatchString("make revise-base TARGET=alpine/3.24") {
		t.Fatal("base revision command blocked")
	}
	if !pattern.MatchString("make dev-generate") {
		t.Fatal("workflow regeneration command blocked")
	}
	for _, command := range []string{"make update TARGET=alpine-3-24-php-8-4;id", "make update TARGET=$(id)", "make update TARGET=alpine-3-24-php-8-4\nmake publish", "make update TARGET=../../secrets", "make publish TARGET=alpine-3-24-php-8-4", "make revise-base TARGET=alpine/3.24;id", "make revise-base TARGET=../../secrets"} {
		if pattern.MatchString(command) {
			t.Fatal("unsafe command allowed", command)
		}
	}
}

func TestToolImageDigestsHaveExactSHA256Length(t *testing.T) {
	pattern := regexp.MustCompile(`@sha256:([^\s]+)`)
	for _, name := range []string{"Dockerfile", "Dockerfile.docs", ".github/workflows/renovate.yml"} {
		data, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		matches := pattern.FindAllStringSubmatch(string(data), -1)
		if len(matches) == 0 {
			t.Fatal("missing tool digest", name)
		}
		for _, match := range matches {
			if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(match[1]) {
				t.Fatalf("%s: malformed image digest %q", name, match[1])
			}
		}
	}
}

func TestJavaDiscoveryDisabled(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	data, err := JSON(Renovate(c))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "jdk21u") || strings.Contains(string(data), "jdk25u") {
		t.Fatal("Java discovery requires reviewed provenance")
	}
}

func TestWeeklyValidationCannotPublishOrBlockReleases(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := Workflow("../..", c)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"cron: '11 3 * * 0'",
		"BUILD_ALL: ${{ github.event_name == 'schedule' || inputs.all || false }}",
		"validate-only: ${{ github.event_name == 'schedule' || (github.event_name == 'workflow_dispatch' && inputs.all == true) }}",
		"format('validation-{0}', github.run_id)",
		"make dev-test-install",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("weekly validation missing %s", required)
		}
	}
	selection, err := Select(context.Background(), config.Environment{"BUILD_ALL": "true"}, "../..")
	if err != nil || len(selection.Targets) != len(c) {
		t.Fatalf("weekly matrix incomplete: %+v %v", selection, err)
	}
}

func TestInstallGateFailsClosed(t *testing.T) {
	data, err := os.ReadFile("../../ci/push.yml.template")
	if err != nil {
		t.Fatal(err)
	}
	gate := strings.Split(strings.Split(string(data), "      - name: Check every selected build and test\n        run: |\n")[1], "\n  docs-build:")[0]
	for _, selected := range []bool{false, true} {
		for _, result := range []string{"success", "failure", "cancelled", "skipped"} {
			targets := `[]`
			if selected {
				targets = `["example"]`
			}
			results := fmt.Sprintf(`{"plan":{"result":"success"},"install-check":{"result":%q},"build-example":{"result":"success"},"test-example":{"result":"success"}}`, result)
			cmd := exec.Command("bash", "-e", "-c", gate)
			cmd.Env = append(os.Environ(), "TARGETS="+targets, "RESULTS="+results, "FORK=false")
			want := selected && result == "success" || !selected && result == "skipped"
			if err := cmd.Run(); (err == nil) != want {
				t.Fatalf("selected=%v install=%s: %v", selected, result, err)
			}
		}
	}
}

func TestRequiredGateReportsPlanningErrors(t *testing.T) {
	data, err := os.ReadFile("../../ci/push.yml.template")
	if err != nil {
		t.Fatal(err)
	}
	gate := strings.Split(strings.Split(string(data), "      - name: Check every selected build and test\n        run: |\n")[1], "\n  docs-build:")[0]
	for _, tc := range []struct{ result, targets, message string }{
		{"failure", "", "Planning failed"},
		{"cancelled", "", "Planning failed"},
		{"success", "", "missing or invalid targets"},
		{"success", "not-json", "missing or invalid targets"},
		{"success", "null", "missing or invalid targets"},
		{"success", `{}`, "missing or invalid targets"},
		{"success", `[1]`, "missing or invalid targets"},
	} {
		cmd := exec.Command("bash", "-e", "-c", gate)
		cmd.Env = append(os.Environ(), "TARGETS="+tc.targets, `RESULTS={"plan":{"result":"`+tc.result+`"}}`)
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(output), tc.message) || strings.Contains(string(output), "invalid JSON text passed") {
			t.Fatalf("%+v: %s (%v)", tc, output, err)
		}
	}
}

func TestDocumentationWaitsForRequiredGate(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := Workflow("../..", c)
	if err != nil {
		t.Fatal(err)
	}
	docs := strings.Split(strings.Split(workflow, "  docs-build:")[1], "  docs-deploy:")[0]
	for _, required := range []string{"needs: required-checks", "always() && !cancelled()", "github.ref == 'refs/heads/main'", "github.event_name != 'pull_request'", "needs.required-checks.result == 'success'", "make docs-packages", "make docs-build", "persist-credentials: false"} {
		if !strings.Contains(docs, required) {
			t.Fatalf("documentation gate missing %s", required)
		}
	}
	if strings.Contains(docs, "workflow_run") {
		t.Fatal("documentation must use this run's checkout")
	}
	deploy := strings.Split(workflow, "  docs-deploy:")[1]
	if !strings.Contains(deploy, "always() && !cancelled()") || !strings.Contains(deploy, "needs.docs-build.result == 'success'") {
		t.Fatal("deployment must tolerate skipped package ancestors but require a successful docs build")
	}
}

func TestWorkflowTemplateIsValidYAML(t *testing.T) {
	data, err := os.ReadFile("../../ci/push.yml.template")
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("Renovate must be able to parse the source template: %v", err)
	}
}
