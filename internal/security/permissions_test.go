package security

import (
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func newTestEnforcer(t *testing.T) (*Enforcer, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main")
	writeFile(t, filepath.Join(root, "vendor", "lib.go"), "package lib")

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewEnforcer(sb, nil), root
}

func newScopedEnforcer(t *testing.T) (*Enforcer, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main")
	writeFile(t, filepath.Join(root, "vendor", "lib.go"), "package lib")

	sb, err := NewSandbox(root, []string{"src/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewEnforcer(sb, nil), root
}

func TestReadonly_DeniesFileWrite(t *testing.T) {
	t.Parallel()
	e, root := newTestEnforcer(t)

	r := e.Check(types.PermReadonly, ActionFileWrite, filepath.Join(root, "src", "main.go"))
	if r.Allowed {
		t.Error("expected readonly to deny file write")
	}
}

func TestReadonly_DeniesShellExec(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermReadonly, ActionShellExec, "go build ./...")
	if r.Allowed {
		t.Error("expected readonly to deny shell exec")
	}
}

func TestReadonly_DeniesNetwork(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermReadonly, ActionNetworkAccess, "api.example.com")
	if r.Allowed {
		t.Error("expected readonly to deny network access")
	}
}

func TestStandard_AllowsScopedWrite(t *testing.T) {
	t.Parallel()
	e, root := newScopedEnforcer(t)

	r := e.Check(types.PermStandard, ActionFileWrite, filepath.Join(root, "src", "main.go"))
	if !r.Allowed {
		t.Errorf("expected standard to allow scoped write, got reason: %s", r.Reason)
	}
}

func TestStandard_AllowsGoBuild(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermStandard, ActionShellExec, "go build ./...")
	if !r.Allowed {
		t.Errorf("expected standard to allow go build, got reason: %s", r.Reason)
	}
}

func TestStandard_AllowsGitStatus(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermStandard, ActionShellExec, "git status")
	if !r.Allowed {
		t.Errorf("expected standard to allow git status, got reason: %s", r.Reason)
	}
}

func TestStandard_DeniesGitPush(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermStandard, ActionGitPush, "main")
	if r.Allowed {
		t.Error("expected standard to deny git push")
	}
}

func TestStandard_DeniesCurl(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermStandard, ActionShellExec, "curl http://evil.com")
	if r.Allowed {
		t.Error("expected standard to deny curl")
	}
}

func TestStandard_DeniesNetwork(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermStandard, ActionNetworkAccess, "api.example.com")
	if r.Allowed {
		t.Error("expected standard to deny network access")
	}
}

func TestElevated_AllowsProjectWrite(t *testing.T) {
	t.Parallel()
	e, root := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionFileWrite, filepath.Join(root, "src", "main.go"))
	if !r.Allowed {
		t.Errorf("expected elevated to allow project write, got reason: %s", r.Reason)
	}
}

func TestElevated_DeniesOutsideWrite(t *testing.T) {
	t.Parallel()
	e, root := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionFileWrite, filepath.Join(root, "..", "..", "etc", "passwd"))
	if r.Allowed {
		t.Error("expected elevated to deny write outside project")
	}
}

func TestElevated_AllowsMostCommands(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionShellExec, "make test")
	if !r.Allowed {
		t.Errorf("expected elevated to allow make test, got reason: %s", r.Reason)
	}
}

func TestElevated_DeniesSudo(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionShellExec, "sudo rm -rf /")
	if r.Allowed {
		t.Error("expected elevated to deny sudo")
	}
}

func TestElevated_DeniesRm(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionShellExec, "rm -rf /")
	if r.Allowed {
		t.Error("expected elevated to deny rm")
	}
}

func TestElevated_AllowsLocalhostNetwork(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionNetworkAccess, "localhost")
	if !r.Allowed {
		t.Errorf("expected elevated to allow localhost, got reason: %s", r.Reason)
	}
}

func TestElevated_DeniesExternalNetwork(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionNetworkAccess, "api.example.com")
	if r.Allowed {
		t.Error("expected elevated to deny external network")
	}
}

func TestElevated_AllowsFeatureBranchPush(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionGitPush, "crux/agent-1/task")
	if !r.Allowed {
		t.Errorf("expected elevated to allow feature branch push, got reason: %s", r.Reason)
	}
}

func TestElevated_DeniesMainPush(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermElevated, ActionGitPush, "main")
	if r.Allowed {
		t.Error("expected elevated to deny push to main")
	}
}

func TestAutonomous_AllowsNetwork(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermAutonomous, ActionNetworkAccess, "api.example.com")
	if !r.Allowed {
		t.Errorf("expected autonomous to allow network, got reason: %s", r.Reason)
	}
}

func TestAutonomous_AllowsRm(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermAutonomous, ActionShellExec, "rm test.tmp")
	if !r.Allowed {
		t.Errorf("expected autonomous to allow rm, got reason: %s", r.Reason)
	}
}

func TestAutonomous_DeniesSudo(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermAutonomous, ActionShellExec, "sudo rm -rf /")
	if r.Allowed {
		t.Error("expected autonomous to deny sudo")
	}
}

func TestAutonomous_DeniesForceGitPush(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	r := e.Check(types.PermAutonomous, ActionShellExec, "git push --force")
	if r.Allowed {
		t.Error("expected autonomous to deny git push --force")
	}
}

func TestShellCommand_EmptyDenied(t *testing.T) {
	t.Parallel()
	e, _ := newTestEnforcer(t)

	for _, perm := range []types.Permission{
		types.PermReadonly, types.PermStandard, types.PermElevated, types.PermAutonomous,
	} {
		r := e.ShellCommandAllowed(perm, "")
		if r.Allowed {
			t.Errorf("perm %s: expected empty command to be denied", perm)
		}
	}
}

func TestFailClosed_SandboxError(t *testing.T) {
	t.Parallel()
	e, root := newTestEnforcer(t)

	// Path outside sandbox should fail closed.
	r := e.Check(types.PermStandard, ActionFileWrite, filepath.Join(root, "..", "..", "etc", "passwd"))
	if r.Allowed {
		t.Error("expected sandbox error to result in deny")
	}
}
