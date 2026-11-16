package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestChildEnvironmentExcludesInjection(t *testing.T) {
	env := childEnvironment([]string{"HOME=/tmp/owned", "HOME=/tmp/second", "TMPDIR=/tmp", "PATH=/malicious", "DYLD_INSERT_LIBRARIES=secret", "HTTP_PROXY=private", "MYGO_AUTOMATION=/foreign", "MY_BILIBILI_E2E_SCENARIO=foreign", "MPV_HOME=private"}, "/owned/socket", "delayed")
	joined := strings.Join(env, "\n")
	for _, bad := range []string{"/malicious", "secret", "private", "foreign", "second"} {
		if strings.Contains(joined, bad) {
			t.Fatal("unsafe environment inherited", bad)
		}
	}
	if !strings.Contains(joined, "MYGO_AUTOMATION=/owned/socket") || !strings.Contains(joined, "MY_BILIBILI_E2E_SCENARIO=delayed") {
		t.Fatal("owned endpoints missing")
	}
}
func TestOwnedProcessHelper(t *testing.T) {
	if os.Getenv("M2_PROCESS_TEST_HELPER") != "yes" {
		return
	}
	time.Sleep(10 * time.Second)
	os.Exit(0)
}
func TestOwnedProcessKillIsReapedAndRetained(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestOwnedProcessHelper$")
	cmd.Env = append(os.Environ(), "M2_PROCESS_TEST_HELPER=yes")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &ownedProcess{cmd: cmd, done: make(chan error, 1)}
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if !p.exited() {
			_ = cmd.Process.Kill()
			p.await(time.Second)
		}
	})
	if p.exited() || p.await(5*time.Millisecond) {
		t.Fatal("live child mistaken for exit")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if !p.await(time.Second) || !p.exited() || p.exit == nil {
		t.Fatal("forced child not reaped with failure")
	}
}
func TestLiveSurvivalRejectsAlreadyExitedProcess(t *testing.T) {
	p := &ownedProcess{reaped: true}
	if err := (&client{}).survive(t.Context(), p, "", &report{}); err == nil {
		t.Fatal("already exited child passed live survival")
	}
}
