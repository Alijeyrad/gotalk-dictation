//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMain(m *testing.M) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		fmt.Println("Docker not available; skipping integration tests:", err)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// findProjectRoot walks up from this file's location until it finds go.mod.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Log("runtime.Caller failed; falling back to os.Getwd")
		dir, _ := os.Getwd()
		return dir
	}
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Log("go.mod not found; falling back to os.Getwd")
	wd, _ := os.Getwd()
	return wd
}

// testScript is the shell script run inside each container.
const testScript = `set -e
cp -r /src/. /workspace/
cd /workspace
cp tests/integration/testdata/alsa_null.conf ~/.asoundrc
# Align go directive with the installed toolchain so GOTOOLCHAIN=local works
# even when go.mod declares a newer minimum version.
GOINSTALLED=$(go version | awk '{print $3}' | sed 's/go//')
go mod edit -go="$GOINSTALLED" -toolchain=none
# Run unit tests (no X11 required).
go test -v -count=1 ./...
# Start Xvfb for X11 integration tests.
Xvfb :99 -screen 0 1280x720x24 &
XVFB_PID=$!
sleep 1
export DISPLAY=:99
# Run X11 integration tests.
go test -v -count=1 -tags x11test ./internal/typing/... ./internal/audio/...
kill $XVFB_PID || true
`

func TestDistroBuildsAndTests(t *testing.T) {
	projectRoot := findProjectRoot(t)

	distros := []struct {
		name           string
		dockerfileDir  string
	}{
		{
			name:          "ubuntu-22.04",
			dockerfileDir: filepath.Join(projectRoot, "tests", "integration", "dockerfiles", "ubuntu-22.04"),
		},
		{
			name:          "fedora-39",
			dockerfileDir: filepath.Join(projectRoot, "tests", "integration", "dockerfiles", "fedora-39"),
		},
		{
			name:          "arch-latest",
			dockerfileDir: filepath.Join(projectRoot, "tests", "integration", "dockerfiles", "arch-latest"),
		},
	}

	for _, d := range distros {
		d := d // capture for closure
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			req := testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:       d.dockerfileDir,
					Dockerfile:    "Dockerfile",
					KeepImage:     true, // cache between runs
					PrintBuildLog: false,
				},
				Mounts: testcontainers.ContainerMounts{
					{
						Source:   testcontainers.GenericBindMountSource{HostPath: projectRoot},
						Target:   testcontainers.ContainerMountTarget("/src"),
						ReadOnly: true,
					},
					{
						Source: testcontainers.GenericVolumeMountSource{Name: "gotalk-gomodcache"},
						Target: testcontainers.ContainerMountTarget("/root/go/pkg/mod"),
					},
				},
				Cmd: []string{"/bin/sh", "-c", testScript},
				WaitingFor: wait.ForExit().WithExitTimeout(15 * time.Minute),
			}

			container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: req,
				Started:          true,
			})
			if err != nil {
				t.Errorf("[%s] failed to start container: %v", d.name, err)
				return
			}
			defer container.Terminate(context.Background()) //nolint:errcheck

			// Collect container logs.
			logReader, logErr := container.Logs(ctx)
			var logOutput string
			if logErr == nil {
				raw, _ := io.ReadAll(logReader)
				logOutput = string(raw)
			}
			t.Logf("[%s] container logs:\n%s", d.name, logOutput)

			// Check exit code.
			state, err := container.State(ctx)
			if err != nil {
				t.Errorf("[%s] failed to get container state: %v", d.name, err)
				return
			}
			if state.ExitCode != 0 {
				// Use Errorf (not FailNow) so all distros run even if one fails.
				t.Errorf("[%s] container exited with code %d\nlogs:\n%s",
					d.name, state.ExitCode, logOutput)
				if strings.Contains(logOutput, "FAIL") {
					t.Logf("[%s] Test failures detected in logs", d.name)
				}
			}
		})
	}
}
