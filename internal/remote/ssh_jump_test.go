package remote

import (
	"os"
	"path/filepath"
	"runtime"
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
	if direct == "" || jump1 == "" || jump2 == "" {
		t.Skip("SSH multiplexing disabled when cache paths exceed socket budget")
	}
	if direct == jump1 || direct == jump2 || jump1 == jump2 {
		t.Fatalf("SSH ControlPath collision: direct=%q jump1=%q jump2=%q", direct, jump1, jump2)
	}
	if strings.Contains(jump1, "a@jump1") {
		t.Errorf("Jump details exposed in control socket path")
	}
	if want := sshControlPath("a@jump1"); jump1 != want {
		t.Errorf("ControlPath is unstable: %q vs %q", jump1, want)
	}
}

// An isolated XDG_CACHE_HOME can be longer than the Unix domain socket limit.
// Jump SSH must still work in this case; only connection pooling is skipped.
func TestSSHLongCachePathDisablesMultiplexing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME semantics tested on Linux")
	}
	dir := filepath.Join(t.TempDir(), "cache", strings.Repeat("very-long-cache-", 8))
	t.Setenv("XDG_CACHE_HOME", dir)
	if cache, err := os.UserCacheDir(); err != nil || cache != dir {
		t.Skipf("custom XDG cache directory unavailable: %q %v", cache, err)
	}
	if got := sshControlPath("jumpuser@192.0.2.10"); got != "" {
		t.Fatalf("unexpected oversized multiplex path: %q", got)
	}
	args := strings.Join(sshArgs(config.Endpoint{Host: "192.0.2.20", User: "developer", Jump: "jumpuser@192.0.2.10"}, false), " ")
	for _, flag := range []string{"-J jumpuser@192.0.2.10", "ControlMaster=no", "ControlPath=none", "ForwardAgent=no"} {
		if !strings.Contains(args, flag) {
			t.Errorf("missing %q from SSH options %q", flag, args)
		}
	}
	if strings.Contains(args, "ControlMaster=auto") {
		t.Fatalf("SSH tried oversized multiplex path: %q", args)
	}
}

func TestSSHControlPathExpandedLengthWithinSocketBudget(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME semantics tested on Linux")
	}
	// Short caches keep efficient multiplexing, while jump routes remain isolated.
	shortDir, err := os.MkdirTemp("/tmp", "r")
	if err != nil {
		t.Skipf("short /tmp directory unavailable: %v", err)
	}
	defer os.RemoveAll(shortDir)
	t.Setenv("XDG_CACHE_HOME", shortDir)
	if cache, err := os.UserCacheDir(); err != nil || cache != shortDir {
		t.Skipf("custom XDG cache directory unavailable: %q %v", cache, err)
	}
	for _, jump := range []string{"", "jumpuser@192.0.2.10"} {
		path := sshControlPath(jump)
		if path == "" {
			t.Fatalf("multiplexing unexpectedly disabled for jump %q", jump)
		}
		actualMax := len(path) - len("%C") + sshControlDigestLen + sshControlTmpSuffixLen
		if actualMax > sshControlSocketMaxSize {
			t.Fatalf("socket path estimated length %d exceeds budget: %q", actualMax, path)
		}
	}
}

func TestSSHControlPathPercentInCacheDisablesMultiplexing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME semantics tested on Linux")
	}
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "%Ccache"))
	if path := sshControlPath("jump"); path != "" {
		t.Fatalf("percent-containing cache path allowed: %q", path)
	}
}
