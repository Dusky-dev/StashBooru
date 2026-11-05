package booru

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// fetchMegaMedia only streams single-file public links. Never execute a shell,
// infer a folder selector, forward credentials, or return provider stderr.
func fetchMegaMedia(ctx context.Context, request MediaRequest, output io.Writer) (MediaInfo, error) {
	var empty MediaInfo
	if !ValidMegaSharedFileURL(request.URL) {
		return empty, fmt.Errorf("unsupported MEGA file link")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := exec.LookPath("megadl"); err != nil {
		return empty, fmt.Errorf("MEGA downloads require megadl (megatools) installed on the Stash server")
	}
	cmd := exec.CommandContext(ctx, "megadl", "--path", "-", "--no-progress", request.URL)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return empty, fmt.Errorf("cannot start MEGA file download")
	}
	if err = cmd.Start(); err != nil {
		return empty, fmt.Errorf("cannot start MEGA file download")
	}
	result, readErr := readValidatedMedia(stdout, request, output, -1)
	if readErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return empty, readErr
	}
	if waitErr != nil || ctx.Err() != nil {
		return empty, fmt.Errorf("MEGA download failed or the transfer quota was exceeded")
	}
	return result, nil
}
