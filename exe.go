package main

import (
	"os"
	"path/filepath"
)

func exe(n string) string {
	// /pkg/main/media-video.ffmpeg.core/bin/...
	p := filepath.Join("/pkg/main/media-video.ffmpeg.core/bin", n)
	if _, err := os.Stat(p); err == nil {
		return p
	}

	// by default exec will perform path lookup, so just return n
	return n
}
