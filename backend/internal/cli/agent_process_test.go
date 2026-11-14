package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type supervisedEnvChildResult struct {
	Config    string
	DataHome  string
	Inherited string
	Input     string
	Args      []string
}

func TestAgentProcessSuperviseAppliesEnvPrefixWithoutAShell(t *testing.T) {
	cfg := setConfigEnv(t)
	srv, capture := activityServer(t, http.StatusOK, `{"ok":true}`)
	writeRunFileFor(t, cfg, srv)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_CONFIG_CONTENT", "inherited config")
	t.Setenv("AO_TEST_SUPERVISED_INHERITED", "inherited value")
	config := `{"agents":{"ao-demo":{"system":"literal & | %PATH% $HOME = 中文"}}}`
	dataHome := filepath.Join(t.TempDir(), "OpenCode data 中文")
	literalArg := "literal & | %PATH% $HOME = value"
	out, errOut, err := executeCLI(t, Deps{
		In:           strings.NewReader("supervised input"),
		ProcessAlive: func(int) bool { return true },
	}, "agent-process", "supervise", "--session", "ao-7", "--launch", "launch-3", "--",
		"env", "AO_TEST_SUPERVISED_ENV_CHILD=1", "OPENCODE_CONFIG_CONTENT=overridden",
		"OPENCODE_CONFIG_CONTENT="+config, "XDG_DATA_HOME="+dataHome,
		exe, "-test.run=^TestAgentProcessSupervisedEnvChild$", "--", literalArg)
	if err != nil {
		t.Fatalf("supervise returned child exit as command failure: %v\nstderr=%s", err, errOut)
	}
	var got supervisedEnvChildResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("child did not start and return its environment: %v\nstdout=%q\nstderr=%q", err, out, errOut)
	}
	want := supervisedEnvChildResult{
		Config: config, DataHome: dataHome, Inherited: "inherited value", Input: "supervised input", Args: []string{literalArg},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("child environment and arguments = %+v, want %+v", got, want)
	}
	if errOut != "supervised stderr" {
		t.Fatalf("stderr = %q, want child output preserved", errOut)
	}
	if os.Getenv("OPENCODE_CONFIG_CONTENT") != "inherited config" {
		t.Fatal("the child's env prefix modified the supervisor environment")
	}
	if capture.path != "/api/v1/sessions/ao-7/activity" {
		t.Fatalf("exit report path = %q, want session activity endpoint", capture.path)
	}
	var report setActivityAPIRequest
	if err := json.Unmarshal([]byte(capture.body), &report); err != nil {
		t.Fatal(err)
	}
	if report != (setActivityAPIRequest{State: "exited", Event: "process-exited", LaunchID: "launch-3"}) {
		t.Fatalf("exit report lost the supervised generation: %+v", report)
	}
}

func TestAgentProcessSupervisedEnvChild(t *testing.T) {
	if os.Getenv("AO_TEST_SUPERVISED_ENV_CHILD") != "1" {
		return
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(supervisedEnvChildResult{
		Config: os.Getenv("OPENCODE_CONFIG_CONTENT"), DataHome: os.Getenv("XDG_DATA_HOME"),
		Inherited: os.Getenv("AO_TEST_SUPERVISED_INHERITED"), Input: string(input), Args: args,
	})
	_, _ = os.Stderr.WriteString("supervised stderr")
	os.Exit(23)
}
