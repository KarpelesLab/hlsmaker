package main

import (
	"flag"
	"os"
	"path/filepath"
)

var (
	ffmpegBin = flag.String("ffmpeg_bin", "", "path to ffmpeg executable")
)

func exe(n string) string {
	if p := *ffmpegBin; p != "" {
		// do a few attempts
		a := filepath.Join(p, n)
		if _, err := os.Stat(a); err == nil {
			return a
		}

		// go back one level
		p = filepath.Dir(p)

		a = filepath.Join(p, n)
		if _, err := os.Stat(a); err == nil {
			return a
		}

		// fallback
	}

	// /pkg/main/media-video.ffmpeg.core/bin/...
	p := filepath.Join("/pkg/main/media-video.ffmpeg.core/bin", n)
	if _, err := os.Stat(p); err == nil {
		return p
	}

	// by default exec will perform path lookup, so just return n
	return n
}
