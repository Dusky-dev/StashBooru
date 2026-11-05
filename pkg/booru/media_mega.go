package booru

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// fetchMegaMedia only streams single-file public links. Never execute a shell,
// infer a folder selector, forward credentials, or return provider stderr.
func fetchMegaMedia(ctx context.Context, request MediaRequest, output io.Writer) (MediaInfo, error) {
	var empty MediaInfo
	if !ValidMegaSharedFileURL(request.URL) {
		return empty, fmt.Errorf("unsupported MEGA file link")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	// Megatools >=1.11 uses a consolidated "megatools dl" executable. Older
	// distributions provide "megadl"; both accept --path - for stdout streaming.
	binary, err := exec.LookPath("megatools")
	args := []string{"dl", "--ignore-config-file", "--path", "-", "--no-progress"}
	if err != nil {
		binary, err = exec.LookPath("megadl")
		args = []string{"--ignore-config-file", "--path", "-", "--no-progress"}
		if err != nil {
			return empty, fmt.Errorf("MEGA downloads require megatools installed on the Stash server")
		}
	}
	// Translate modern file links to the legacy equivalent for older helpers.
	// Revalidate before constructing any subprocess argument.
	u, _ := url.Parse(request.URL)
	link := request.URL
	if strings.HasPrefix(u.Path, "/file/") {
		link = "https://mega.nz/#!" + strings.TrimPrefix(u.Path, "/file/") + "!" + u.Fragment
	}
	args = append(args, link)
	cmd := exec.CommandContext(ctx, binary, args...)
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
