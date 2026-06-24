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

const defaultMCPTimeout = 2 * time.Minute

func installMCP(item model.PlanItem) model.PlanItem {
	if item.Action == "record" {
		item.Status = "recorded"
		return item
	}

	needsInstall := item.Action == "native-install" || item.Action == "native-install-and-merge"
	if needsInstall && item.Command != "" {
		cwd := "."
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultMCPTimeout)
		defer cancel()

		var cmd *exec.Cmd
		if item.Method == "npm" || item.Method == "shell" || item.Method == "" {
			cmd = exec.CommandContext(ctx, "sh", "-c", item.Command)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", item.Command)
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
				item.Reason = fmt.Sprintf("MCP install command timed out after %s", defaultMCPTimeout)
			}
		}
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
			return item
		}
	}

	if item.Action == "merge" || item.Action == "native-install-and-merge" {
		mergeItem := installMerge(item)
		mergeItem.Stdout = item.Stdout
		mergeItem.Stderr = item.Stderr
		mergeItem.ExitCode = item.ExitCode
		mergeItem.Command = item.Command
		return mergeItem
	}

	item.Status = "installed"
	return item
}

func uninstallMCP(item model.PlanItem, executePlugins bool) model.PlanItem {
	if !executePlugins {
		item.Status = "pending"
		item.Reason = "MCP uninstall command execution requires --execute-mcps"
		return item
	}
	if item.UninstallCommand == "" {
		item.Status = "pending"
		item.Reason = "MCP uninstall command is not specified"
		return item
	}

	cwd := "."
	if wd, err := os.Getwd(); err == nil {
		cwd = wd
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultMCPTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", item.UninstallCommand)
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
			item.Reason = fmt.Sprintf("MCP uninstall command timed out after %s", defaultMCPTimeout)
		}
	}
	item.Command = item.UninstallCommand
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
		item.Status = "uninstalled"
	}
	return item
}
