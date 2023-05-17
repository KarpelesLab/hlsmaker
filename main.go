package main

import (
	"flag"
	"log"
	"os"

	"github.com/KarpelesLab/runutil"
)

var (
	inputFile = flag.String("in", "", "input file")
	encKey    = flag.String("key", "", "encryption key (16 bytes, hexadecimal)")
)

func main() {
	flag.Parse()

	// take input video file (as param to ffmpeg or ffprobe) and generate a video file
	if inputFile == nil || *inputFile == "" {
		log.Printf("Syntax: %s -in filename [-key key]", os.Args[0])
		os.Exit(1)
		return
	}

	// perform ffprobe
	var info *ffprobeInfo
	err := runutil.RunJson(&info, "/pkg/main/media-video.ffmpeg.core/bin/ffprobe", "-print_format", "json", "-hide_banner", "-loglevel", "quiet", "-show_format", "-show_streams", "-show_chapters", *inputFile)
	if err != nil {
		log.Printf("ffprobe failed: %s", err)
		os.Exit(1)
		return
	}

	video := info.video()
	audio := info.audio()
	if video == nil || audio == nil {
		log.Printf("video or audio track missing")
		os.Exit(1)
		return
	}

	log.Printf("input: video stream format %s %dx%d, audio format %s %d Hz", video.CodecName, video.Width, video.Height, audio.CodecName, audio.SampleRate)
}
