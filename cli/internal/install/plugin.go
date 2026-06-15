package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/agent-packs/cli/internal/model"
)

const defaultPluginTimeout = 2 * time.Minute

func installPlugin(item model.PlanItem, executePlugins bool) model.PlanItem {
	if item.Action == "reference" {
		item.Status = "referenced"
		item.Reason = "referenced from source; not copied into target"
		return item
	}
	if !executePlugins {
		item.Status = "pending"
		item.Reason = "plugin command execution requires --execute-plugins"
		return item
	}
	execArgs, command, err := buildPluginExec(item)
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	return runPluginCommand(item, execArgs, command, "installed")
}

func uninstallPlugin(item model.PlanItem, executePlugins bool) model.PlanItem {
	if !executePlugins {
		item.Status = "pending"
		item.Reason = "plugin uninstall command execution requires --execute-plugins"
		return item
	}
	execArgs, command, err := buildPluginUninstallExec(item)
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	return runPluginCommand(item, execArgs, command, "uninstalled")
}

// buildPluginExec returns either structured args (for direct exec, avoiding shell injection)
// or a pre-built command string (for sh -c when the command comes verbatim from the manifest).
// If execArgs is non-empty, use direct exec; otherwise use sh -c with command.
func buildPluginExec(item model.PlanItem) (execArgs []string, command string, err error) {
	switch item.Method {
	case "claude-marketplace":
		if item.Command != "" {
			return nil, item.Command, nil
		}
		if item.Package == "" || item.Marketplace == "" {
			return nil, "", fmt.Errorf("claude-marketplace plugin requires package and marketplace")
		}
		// Use direct exec to avoid shell injection on Package/Marketplace values.
		return []string{"claude", "plugin", "install", item.Package + "@" + item.Marketplace}, "", nil
	case "manual":
		if item.Command == "" {
			return nil, "", fmt.Errorf("plugin install command is not specified")
		}
		return nil, item.Command, nil
	default:
		if item.Command == "" {
			return nil, "", fmt.Errorf("plugin install command is not specified")
		}
		return nil, item.Command, nil
	}
}

func buildPluginUninstallExec(item model.PlanItem) (execArgs []string, command string, err error) {
	if item.UninstallCommand != "" {
		return nil, item.UninstallCommand, nil
	}
	switch item.Method {
	case "claude-marketplace":
		if item.Package == "" || item.Marketplace == "" {
			return nil, "", fmt.Errorf("claude-marketplace plugin requires package and marketplace")
		}
		return []string{"claude", "plugin", "uninstall", item.Package + "@" + item.Marketplace}, "", nil
	default:
		return nil, "", fmt.Errorf("plugin uninstall command is not specified")
	}
}

func runPluginCommand(item model.PlanItem, execArgs []string, command, successStatus string) model.PlanItem {
	cwd := pluginWorkingDirectory(item)
	ctx, cancel := context.WithTimeout(context.Background(), defaultPluginTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if len(execArgs) > 0 {
		cmd = exec.CommandContext(ctx, execArgs[0], execArgs[1:]...)
		command = strings.Join(execArgs, " ")
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			item.Reason = fmt.Sprintf("plugin command timed out after %s", defaultPluginTimeout)
		}
	}
	item.Command = command
	item.ExitCode = &exitCode
	item.Stdout = strings.TrimSpace(stdout.String())
	item.Stderr = strings.TrimSpace(stderr.String())
	if err != nil {
		if item.Reason == "" {
			item.Reason = strings.TrimSpace(stderr.String())
			if item.Reason == "" {
				item.Reason = err.Error()
			}
		}
		item.Status = "failed"
	} else {
		item.Status = successStatus
	}
	return item
}

func pluginWorkingDirectory(item model.PlanItem) string {
	if dir := os.Getenv("AGENT_PACKS_PLUGIN_CWD"); dir != "" {
		return dir
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
