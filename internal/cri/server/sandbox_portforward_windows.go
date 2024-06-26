/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package server

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"k8s.io/utils/exec"

	sandboxstore "github.com/containerd/containerd/v2/internal/cri/store/sandbox"
	cioutil "github.com/containerd/containerd/v2/pkg/ioutil"
)

func (c *criService) portForward(ctx context.Context, id string, port int32, stream io.ReadWriter) error {
	stdout := cioutil.NewNopWriteCloser(stream)
	stderrBuffer := new(bytes.Buffer)
	stderr := cioutil.NewNopWriteCloser(stderrBuffer)
	// localhost is resolved to 127.0.0.1 in ipv4, and ::1 in ipv6.
	// Explicitly using ipv4 IP address in here to avoid flakiness.
	cmd := []string{"wincat.exe", "127.0.0.1", fmt.Sprint(port)}
	err := c.execInSandbox(ctx, id, cmd, stream, stdout, stderr)
	if err != nil {
		return fmt.Errorf("failed to execute port forward in sandbox: %s: %w", stderrBuffer.String(), err)
	}
	return nil
}

func (c *criService) execInSandbox(ctx context.Context, sandboxID string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser) error {
	// Get sandbox from our sandbox store.
	sb, err := c.sandboxStore.Get(sandboxID)
	if err != nil {
		return fmt.Errorf("failed to find sandbox %q in store: %w", sandboxID, err)
	}

	// Check the sandbox state
	state := sb.Status.Get().State
	if state != sandboxstore.StateReady {
		return fmt.Errorf("sandbox is in %s state", fmt.Sprint(state))
	}

	opts := execOptions{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		tty:    false,
		resize: nil,
	}
	exitCode, err := c.execInternal(ctx, sb.Container, sandboxID, opts)
	if err != nil {
		return fmt.Errorf("failed to exec in sandbox: %w", err)
	}
	if *exitCode == 0 {
		return nil
	}
	return &exec.CodeExitError{
		Err:  fmt.Errorf("error executing command %v, exit code %d", cmd, *exitCode),
		Code: int(*exitCode),
	}
}
