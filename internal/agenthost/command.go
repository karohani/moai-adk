package agenthost

import (
	"fmt"
	"strings"
)

// LaunchMode selects whether a host should open an interactive session or run
// a non-interactive task command.
type LaunchMode string

const (
	LaunchInteractive LaunchMode = "interactive"
	LaunchExec        LaunchMode = "exec"
)

// LaunchRequest is the host-neutral input used to calculate an executable argv.
// It intentionally contains no shell string so tests can validate launch
// behavior without executing an external coding-agent binary.
type LaunchRequest struct {
	Host        Host
	ProjectRoot string
	Mode        LaunchMode
	Role        string
	Agent       string
	Model       string
	Prompt      string
	Profile     string
	Sandbox     string
	Approval    string
	Config      string
	Session     string
	Attach      bool
	Continue    bool
	Auto        bool
	ExtraArgs   []string
}

// LaunchCommand is a structured host command. Argv[0] is the executable.
type LaunchCommand struct {
	Host  Host              `json:"host"`
	Cwd   string            `json:"cwd,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
	Argv  []string          `json:"argv"`
	Notes []string          `json:"notes,omitempty"`
}

// String renders the argv for human-facing dry-run output. It is descriptive,
// not shell-escaped output intended for execution.
func (c LaunchCommand) String() string {
	return strings.Join(c.Argv, " ")
}

// BuildLaunchCommand calculates a native launch command for the requested host.
func BuildLaunchCommand(req LaunchRequest) (LaunchCommand, error) {
	host, err := ParseHost(string(req.Host))
	if err != nil {
		return LaunchCommand{}, err
	}
	req.Host = host
	if req.Mode == "" {
		req.Mode = LaunchInteractive
	}

	switch host {
	case HostClaude:
		return buildClaudeCommand(req)
	case HostCodex:
		return buildCodexCommand(req)
	case HostOpenCode:
		return buildOpenCodeCommand(req)
	default:
		return LaunchCommand{}, fmt.Errorf("unsupported host %q", host)
	}
}

func buildClaudeCommand(req LaunchRequest) (LaunchCommand, error) {
	argv := []string{"claude"}
	if req.Model != "" {
		argv = append(argv, "--model", req.Model)
	}
	if req.Profile != "" {
		argv = append(argv, "--profile", req.Profile)
	}
	if req.Prompt != "" {
		argv = append(argv, req.Prompt)
	}
	argv = append(argv, req.ExtraArgs...)
	return LaunchCommand{
		Host: HostClaude,
		Cwd:  req.ProjectRoot,
		Argv: argv,
	}, nil
}

func buildCodexCommand(req LaunchRequest) (LaunchCommand, error) {
	argv := []string{"codex"}
	if req.Mode == LaunchExec {
		argv = append(argv, "exec")
	}
	if req.ProjectRoot != "" {
		argv = append(argv, "--cd", req.ProjectRoot)
	}
	if req.Model != "" {
		argv = append(argv, "--model", req.Model)
	}
	if req.Profile != "" {
		argv = append(argv, "--profile", req.Profile)
	}
	if req.Sandbox != "" {
		argv = append(argv, "--sandbox", req.Sandbox)
	}
	if req.Approval != "" {
		argv = append(argv, "--ask-for-approval", req.Approval)
	}
	if req.Config != "" {
		argv = append(argv, "--config", req.Config)
	}
	if req.Prompt != "" {
		argv = append(argv, req.Prompt)
	}
	argv = append(argv, req.ExtraArgs...)
	return LaunchCommand{
		Host: HostCodex,
		Cwd:  req.ProjectRoot,
		Argv: argv,
	}, nil
}

func buildOpenCodeCommand(req LaunchRequest) (LaunchCommand, error) {
	argv := []string{"opencode"}
	if req.Mode == LaunchExec {
		argv = append(argv, "run")
	}
	if req.ProjectRoot != "" {
		argv = append(argv, "--dir", req.ProjectRoot)
	}
	if req.Agent != "" {
		argv = append(argv, "--agent", req.Agent)
	} else if req.Role != "" {
		argv = append(argv, "--agent", req.Role)
	}
	if req.Model != "" {
		argv = append(argv, "--model", req.Model)
	}
	if req.Session != "" {
		argv = append(argv, "--session", req.Session)
	}
	if req.Continue {
		argv = append(argv, "--continue")
	}
	if req.Attach {
		argv = append(argv, "--attach")
	}
	if req.Auto {
		argv = append(argv, "--auto")
	}
	if req.Prompt != "" {
		argv = append(argv, req.Prompt)
	}
	argv = append(argv, req.ExtraArgs...)
	return LaunchCommand{
		Host: HostOpenCode,
		Cwd:  req.ProjectRoot,
		Argv: argv,
	}, nil
}
