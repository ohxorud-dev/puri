package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

type LanguageConfig struct {
	Image    string
	FileName string
	BuildCmd string
	RunCmd   string
}

var languageConfigs = map[string]LanguageConfig{
	"LANGUAGE_CPP": {
		Image:    "gcc:14",
		FileName: "main.cpp",
		BuildCmd: "g++ -std=c++20 -O2 -o main main.cpp",
		RunCmd:   "./main",
	},
	"LANGUAGE_PYTHON": {
		Image:    "python:3.13-slim",
		FileName: "main.py",
		RunCmd:   "python main.py",
	},
	"LANGUAGE_JAVA": {
		Image:    "amazoncorretto:21",
		FileName: "Main.java",
		BuildCmd: "javac Main.java",
		RunCmd:   "java Main",
	},
	"LANGUAGE_GO": {
		Image:    "golang:1.25",
		FileName: "main.go",
		RunCmd:   "go run main.go",
	},
	"LANGUAGE_JAVASCRIPT": {
		Image:    "node:22-slim",
		FileName: "main.js",
		RunCmd:   "node main.js",
	},
	"LANGUAGE_RUST": {
		Image:    "rust:1.95-slim-trixie",
		FileName: "main.rs",
		BuildCmd: "rustc -O main.rs",
		RunCmd:   "./main",
	},
}

type Result struct {
	Output          string
	ExecutionTimeMs int32
	MemoryUsageKb   int32
	Result          string
	Error           error
}

type DockerRunner struct {
	cli        *client.Client
	pullLocks  sync.Map // imageName -> *sync.Mutex (one pull at a time per image)
	pulledOnce sync.Map // imageName -> struct{} (cached after successful verify)
}

func NewDockerRunner() (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("docker daemon not reachable: %w", err)
	}

	return &DockerRunner{cli: cli}, nil
}

func (r *DockerRunner) ensureImage(ctx context.Context, img string) error {
	if _, ok := r.pulledOnce.Load(img); ok {
		return nil
	}
	lockIface, _ := r.pullLocks.LoadOrStore(img, &sync.Mutex{})
	lock := lockIface.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	if _, ok := r.pulledOnce.Load(img); ok {
		return nil
	}

	if _, _, err := r.cli.ImageInspectWithRaw(ctx, img); err == nil {
		r.pulledOnce.Store(img, struct{}{})
		return nil
	}

	pullCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	rc, err := r.cli.ImagePull(pullCtx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull %s: %w", img, err)
	}
	defer rc.Close()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("image pull drain %s: %w", img, err)
	}
	r.pulledOnce.Store(img, struct{}{})
	return nil
}

func (r *DockerRunner) Run(ctx context.Context, language, sourceCode, input, timeLimit, memoryLimit string) Result {
	langCfg, ok := languageConfigs[language]
	if !ok {
		return Result{Result: "COMPILATION_ERROR", Error: fmt.Errorf("unsupported language: %s", language)}
	}

	if err := r.ensureImage(ctx, langCfg.Image); err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: err}
	}

	// Use /tmp/puri-runs so path is shared with host when running inside Docker
	_ = os.MkdirAll("/tmp/puri-runs", 0755)
	tmpDir, err := os.MkdirTemp("/tmp/puri-runs", "puri-run-*")
	if err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("failed to create temp dir: %w", err)}
	}
	defer os.RemoveAll(tmpDir)
	if err := os.Chmod(tmpDir, 0777); err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("failed to chmod temp dir: %w", err)}
	}

	srcPath := filepath.Join(tmpDir, langCfg.FileName)
	if err := os.WriteFile(srcPath, []byte(sourceCode), 0644); err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("failed to write source: %w", err)}
	}

	inputPath := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("failed to write input: %w", err)}
	}

	timeLimitSec := parseTimeLimit(timeLimit)
	problemMemoryLimitKb := parseMemoryLimit(memoryLimit) / 1024   // problem limit in KB
	containerMemoryBytes := parseMemoryLimit(memoryLimit) + 500*1024*1024 // problem + 500MB buffer

	var cmdParts []string
	if langCfg.BuildCmd != "" {
		cmdParts = append(cmdParts, langCfg.BuildCmd)
	}

	// Runner script: measure execution time + peak memory (VmRSS polling)
	runnerScript := fmt.Sprintf(`#!/bin/sh
timeout --signal=KILL %d %s < input.txt &
PID=$!
MAXMEM=0
while kill -0 $PID 2>/dev/null; do
  MEM=$(awk '/VmRSS/{print $2}' /proc/$PID/status 2>/dev/null || echo 0)
  if [ -n "$MEM" ] && [ "$MEM" -gt "$MAXMEM" ] 2>/dev/null; then
    MAXMEM=$MEM
  fi
  sleep 0.005
done
wait $PID
EXIT=$?
echo $MAXMEM > /workspace/exec_mem.txt
exit $EXIT
`, timeLimitSec, langCfg.RunCmd)

	if err := os.WriteFile(filepath.Join(tmpDir, "runner.sh"), []byte(runnerScript), 0755); err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("failed to write runner script: %w", err)}
	}

	cmdParts = append(cmdParts,
		`START=$(date +%s%3N) && sh runner.sh; EXIT=$?; END=$(date +%s%3N) && echo $((END - START)) > /workspace/exec_time.txt && exit $EXIT`)
	cmd := strings.Join(cmdParts, " && ")

	config := &container.Config{
		Image:           langCfg.Image,
		Cmd:             []string{"sh", "-c", cmd},
		WorkingDir:      "/workspace",
		User:            "nobody",
		Env:             []string{"HOME=/tmp"},
		AttachStdout:    true,
		AttachStderr:    true,
		Tty:             false,
		NetworkDisabled: true,
	}

	pidsLimit := int64(64)
	hostConfig := &container.HostConfig{
		AutoRemove:     true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		ReadonlyRootfs: true,
		Tmpfs: map[string]string{
			"/tmp": "rw,size=64m",
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: tmpDir,
				Target: "/workspace",
			},
		},
		Resources: container.Resources{
			Memory:    containerMemoryBytes,
			NanoCPUs:  1_000_000_000,
			PidsLimit: &pidsLimit,
		},
	}

	containerResp, err := r.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("failed to create container: %w", err)}
	}
	containerID := containerResp.ID

	if err := r.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("failed to start container: %w", err)}
	}

	statusCh, errCh := r.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			_ = r.cli.ContainerKill(context.Background(), containerID, "KILL")
			return Result{Result: "RUNTIME_ERROR", Error: fmt.Errorf("container wait error: %w", err)}
		}
	case status := <-statusCh:
		out, _ := r.cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
		outputBytes := make([]byte, 0, 65536)
		buf := make([]byte, 4096)
		for {
			n, err := out.Read(buf)
			if n > 0 {
				outputBytes = append(outputBytes, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		out.Close()

		cleanOutput := stripDockerStreamHeaders(outputBytes)

		// Read actual execution time and peak memory from inside the container
		var executionTimeMs int32
		var memoryUsageKb int32
		if timeData, err := os.ReadFile(filepath.Join(tmpDir, "exec_time.txt")); err == nil {
			if ms, err := strconv.Atoi(strings.TrimSpace(string(timeData))); err == nil {
				executionTimeMs = int32(ms)
			}
		}
		if memData, err := os.ReadFile(filepath.Join(tmpDir, "exec_mem.txt")); err == nil {
			if kb, err := strconv.Atoi(strings.TrimSpace(string(memData))); err == nil {
				memoryUsageKb = int32(kb)
			}
		}

		if status.StatusCode == 124 || status.StatusCode == 137 {
			return Result{Output: cleanOutput, ExecutionTimeMs: executionTimeMs, MemoryUsageKb: memoryUsageKb, Result: "TIME_LIMIT_EXCEEDED"}
		}

		// Check memory against problem limit (container has +500MB buffer)
		if memoryUsageKb > int32(problemMemoryLimitKb) {
			return Result{Output: cleanOutput, ExecutionTimeMs: executionTimeMs, MemoryUsageKb: memoryUsageKb, Result: "MEMORY_LIMIT_EXCEEDED"}
		}

		if status.StatusCode != 0 {
			return Result{Output: cleanOutput, ExecutionTimeMs: executionTimeMs, MemoryUsageKb: memoryUsageKb, Result: "RUNTIME_ERROR"}
		}

		return Result{Output: cleanOutput, ExecutionTimeMs: executionTimeMs, MemoryUsageKb: memoryUsageKb, Result: "ACCEPTED"}
	case <-ctx.Done():
		_ = r.cli.ContainerKill(context.Background(), containerID, "KILL")
		return Result{Result: "TIME_LIMIT_EXCEEDED", Error: fmt.Errorf("hard timeout exceeded")}
	}
	return Result{Result: "INTERNAL_ERROR", Error: fmt.Errorf("unexpected end of run")}
}

func parseTimeLimit(limit string) int {
	limit = strings.TrimSpace(limit)
	limit = strings.ReplaceAll(limit, "초", "")
	limit = strings.TrimSpace(limit)
	if sec, err := strconv.ParseFloat(limit, 64); err == nil {
		return int(sec) + 1
	}
	return 3
}

func parseMemoryLimit(limit string) int64 {
	limit = strings.ToUpper(strings.TrimSpace(limit))
	limit = strings.ReplaceAll(limit, " ", "")

	multiplier := int64(1)
	if strings.HasSuffix(limit, "MB") {
		multiplier = 1024 * 1024
		limit = strings.TrimSuffix(limit, "MB")
	} else if strings.HasSuffix(limit, "GB") {
		multiplier = 1024 * 1024 * 1024
		limit = strings.TrimSuffix(limit, "GB")
	} else if strings.HasSuffix(limit, "KB") {
		multiplier = 1024
		limit = strings.TrimSuffix(limit, "KB")
	}

	if val, err := strconv.ParseInt(strings.TrimSpace(limit), 10, 64); err == nil {
		return val * multiplier
	}
	return 256 * 1024 * 1024
}

func stripDockerStreamHeaders(data []byte) string {
	var result []byte
	i := 0
	for i < len(data) {
		if i+8 > len(data) {
			result = append(result, data[i:]...)
			break
		}
		header := data[i : i+8]
		streamType := header[0]
		if streamType != 1 && streamType != 2 {
			result = append(result, data[i:]...)
			break
		}
		// Docker stream header uses big-endian for payload length
		payloadLen := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])
		if i+8+payloadLen > len(data) {
			result = append(result, data[i+8:]...)
			break
		}
		result = append(result, data[i+8:i+8+payloadLen]...)
		i += 8 + payloadLen
	}
	// Ensure valid UTF-8 for protobuf string fields
	s := strings.TrimSpace(string(result))
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\uFFFD")
	}
	return s
}
