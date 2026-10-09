package remote

import (
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
)

func TestSSHArgsProxyJumpAndNoAgentForwarding(t *testing.T) {
	target := config.Endpoint{Host: "private.internal", User: "test", Port: 2222, Jump: "user@bastion:2200,bastion2"}
	args := sshArgs(target, false)
	got := strings.Join(args, " ")
	for _, want := range []string{"-J user@bastion:2200,bastion2", "-p 2222", "ForwardAgent=no"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in ssh args %q", want, got)
		}
	}
	if strings.Contains(got, "-A") {
		t.Fatalf("unexpected agent forwarding: %q", got)
	}
	if strings.Contains(got, "ProxyCommand") {
		t.Fatalf("must not use shell ProxyCommand: %q", got)
	}
}

func TestSSHArgsDirectUnchanged(t *testing.T) {
	target := config.Endpoint{Host: "host", Port: 22}
	got := strings.Join(sshArgs(target, false), " ")
	if strings.Contains(got, "-J ") || strings.Contains(got, "ForwardAgent=no") {
		t.Fatalf("direct SSH flags unexpectedly changed: %q", got)
	}
}

func TestSSHMultiplexIsolationPerJump(t *testing.T) {
	direct := sshControlPath("")
	jump1 := sshControlPath("a@jump1")
	jump2 := sshControlPath("b@jump2")
	if direct == "" {
		t.Skip("cache directory unavailable")
	}
	if jump1 == "" || jump2 == "" || direct == jump1 || direct == jump2 || jump1 == jump2 {
		t.Fatalf("SSH ControlPath collision: direct=%q jump1=%q jump2=%q", direct, jump1, jump2)
	}
	if strings.Contains(jump1, "a@jump1") {
		t.Errorf("Jump details exposed in control socket path")
	}
	if want := sshControlPath("a@jump1"); jump1 != want {
		t.Errorf("ControlPath is unstable: %q vs %q", jump1, want)
	}
}
