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

	"github.com/sandeshh/agent-packs/cli/internal/model"
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
	command, err := buildPluginCommand(item)
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	cwd := pluginWorkingDirectory(item)
	ctx, cancel := context.WithTimeout(context.Background(), defaultPluginTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
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
		item.Status = "installed"
	}
	return item
}

func buildPluginCommand(item model.PlanItem) (string, error) {
	switch item.Method {
	case "claude-marketplace":
		if item.Command != "" {
			return item.Command, nil
		}
		if item.Package == "" || item.Marketplace == "" {
			return "", fmt.Errorf("claude-marketplace plugin requires package and marketplace")
		}
		return fmt.Sprintf("claude plugin install %s@%s", item.Package, item.Marketplace), nil
	case "manual":
		if item.Command == "" {
			return "", fmt.Errorf("plugin install command is not specified")
		}
		return item.Command, nil
	default:
		if item.Command == "" {
			return "", fmt.Errorf("plugin install command is not specified")
		}
		return item.Command, nil
	}
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
