package mediaconvert

import (
	"context"
	"fmt"
	"time"
)

// SelectWorker probes the remote before starting local codec/hardware work.
func SelectWorker(ctx context.Context, mode string, local, remote Client) (Client, Capabilities, string, error) {
	return selectWorker(ctx, mode, local, remote, func(ctx context.Context, c Client) (Capabilities, error) {
		return c.Capabilities(ctx)
	})
}

func selectWorker(ctx context.Context, mode string, local, remote Client, probe func(context.Context, Client) (Capabilities, error)) (Client, Capabilities, string, error) {
	var empty Capabilities
	if mode != "" && mode != "auto" && mode != "local" && mode != "remote" {
		return local, empty, "", fmt.Errorf("unknown conversion backend")
	}
	if mode == "remote" {
		if remote.URL == "" {
			return local, empty, "", fmt.Errorf("configure the remote tagging worker URL in System settings first")
		}
		caps, err := probe(ctx, remote)
		return remote, caps, "", err
	}
	notice := ""
	if mode != "local" && remote.URL != "" {
		remoteCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		caps, err := probe(remoteCtx, remote)
		cancel()
		if ctx.Err() != nil {
			return local, empty, "", ctx.Err()
		}
		if err == nil {
			for _, format := range caps.Formats {
				if format.Available {
					return remote, caps, "", nil
				}
			}
		}
		notice = "Remote converter unavailable; using this StashBooru server."
	}
	caps, err := probe(ctx, local)
	return local, caps, notice, err
}
