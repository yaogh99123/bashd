package server

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
)

func getDocumentation(command string) string {
	var documentation string
	if slices.Contains(append(BASH_KEYWORDS[:], BASH_BUILTINS[:]...), command) {
		documentation = runHelp(command)
	} else {
		documentation = runMan(command)
	}

	doc := strings.Trim(documentation, "\n")
	if doc == "" {
		return " " // 返回一个空格字符，避免 null/empty 导致的 LSP 异常
	}
	return doc
}

func runMan(command string) string {
	manPath, err := exec.LookPath("man")
	if err != nil {
		manPath = "/usr/bin/man" // 兜底路径
	}
	colPath, err := exec.LookPath("col")
	if err != nil {
		colPath = "/usr/bin/col"
	}

	manCmd := exec.Command(manPath, "-p", "cat", command)
	colCmd := exec.Command(colPath, "-bx")

	manOutput, err := runPipe(manCmd, colCmd)
	if err != nil {
		slog.Error("Error running pipe", "err", err)
	}
	return manOutput
}

func runHelp(command string) string {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		bashPath = "/bin/bash" // 兜底路径
	}
	colPath, err := exec.LookPath("col")
	if err != nil {
		colPath = "/usr/bin/col"
	}

	helpCmd := exec.Command(bashPath, "-c", fmt.Sprintf("help %s", command))
	colCmd := exec.Command(colPath, "-bx")

	helpOutput, err := runPipe(helpCmd, colCmd)
	if err != nil {
		slog.Error("Error running pipe", "err", err)
	}
	return helpOutput
}

func runPipe(cmd1, cmd2 *exec.Cmd) (string, error) {
	pipeReader, pipeWriter := io.Pipe()
	cmd1.Stdout = pipeWriter
	cmd2.Stdin = pipeReader

	var out bytes.Buffer
	cmd2.Stdout = &out

	if err := cmd1.Start(); err != nil {
		return "", fmt.Errorf("Error running command %v", cmd1)
	}
	if err := cmd2.Start(); err != nil {
		return "", fmt.Errorf("Error running command %v", cmd2)
	}

	go func() {
		defer pipeWriter.Close()
		cmd1.Wait()
	}()

	if err := cmd2.Wait(); err != nil {
		return "", fmt.Errorf("Error waiting for command %v", cmd2)
	}

	return out.String(), nil
}
