package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/KarpelesLab/runutil"
)

var (
	maxStreams   = flag.Int("max_streams", 16, "set the maximum number of video streams to include")
	softwareMode = flag.Bool("software", false, "enable software encoding")
)

func encodeVideo(d string) error {
	// perform ffprobe
	var info *ffprobeInfo
	err := runutil.RunJson(&info, "/pkg/main/media-video.ffmpeg.core/bin/ffprobe", "-print_format", "json", "-hide_banner", "-loglevel", "quiet", "-show_format", "-show_streams", "-show_chapters", *inputFile)
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	video := info.video()
	audio := info.audio()
	if video == nil || audio == nil {
		return fmt.Errorf("video or audio track missing")
	}

	log.Printf("input: video stream format %s %dx%d, audio format %s %d Hz", video.CodecName, video.Width, video.Height, audio.CodecName, audio.SampleRate)

	siz := &vsize{w: video.Width, h: video.Height}

	// generate variant sizes
	variants := []*vsize{siz}
	for siz = siz.smaller(); siz != nil; siz = siz.smaller() {
		if len(variants) >= *maxStreams {
			break
		}
		variants = append(variants, siz)
	}

	log.Printf("will be generating the following sizes: %v", variants)

	// prepare the command line
	args := []string{
		"-i", *inputFile, "-hide_banner", "-loglevel", "warning",
	}

	// prepare filter_complex
	flt := fmt.Sprintf("[0:v]split=%d", len(variants))
	for n := range variants {
		if n == 0 {
			flt += "[v0]"
			continue
		}
		flt += fmt.Sprintf("[vin%d]", n)
	}
	for n, s := range variants {
		if n == 0 {
			// n==0 means original size
			continue
		}
		flt += fmt.Sprintf(";[vin%d]%s[v%d]", n, s.Scale(), n)
	}
	args = append(args, "-filter_complex", flt)

	// map filters
	rate := video.FrameRate.Value()

	// force good framerate values
	if rate > 60 {
		rate = 60
	} else if rate < 10 {
		rate = 10
	}
	for n, s := range variants {
		ns := strconv.Itoa(n)
		bitrateInt := s.bitrate(rate, 0.1)
		br := strconv.FormatUint(bitrateInt, 10) // we use 0.1 bit per pixel for now
		if *softwareMode {
			args = append(args,
				"-map", fmt.Sprintf("[v%d]", n),
				"-c:v:"+ns, "libx264",
				"-x264-params", "nal-hrd=cbr:force-cfr=1",
				"-b:v:"+ns, br,
				"-maxrate:v:"+ns, br,
				"-minrate:v:"+ns, br,
				"-bufsize", strconv.FormatUint(bitrateInt*2, 10),
				"-preset", "slow",
				"-g", "48",
				"-sc_threshold", "0",
				"-keyint_min", "48",
			)
		} else {
			args = append(args,
				"-map", fmt.Sprintf("[v%d]", n),
				"-c:v:"+ns, "h264_nvenc",
				"-profile:v:"+ns, "high",
				"-preset", "p5",
				"-b:v:"+ns, br,
				"-maxrate:v:"+ns, br,
			)
		}
		// audio
		args = append(args,
			"-map", "a:0",
			"-c:a:"+ns, "aac",
			"-b:a:"+ns, "96k", // TODO variable audio bitrate
			"-ac:a:"+ns, "2", // do we need to make it stereo?
		)
	}

	var varStreamMap []string
	for n := range variants {
		varStreamMap = append(varStreamMap, fmt.Sprintf("v:%d,a:%d", n, n))
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "5",
		// -hls_enc 1
		// -hls_enc_key "42424242424242424242424242424242"
		// -hls_enc_key_url 'key.ts'
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments+single_file",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", "stream_%v.ts",
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", strings.Join(varStreamMap, " "),
		"stream_%v.m3u8",
	)

	log.Printf("ffmpeg arguments: %v", args)
	c := exec.Command("/pkg/main/media-video.ffmpeg.core/bin/ffmpeg", args...)
	c.Dir = d // set to run in temp dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err = c.Run()
	if err != nil {
		// [h264_nvenc @ 0x558dde66e480] OpenEncodeSessionEx failed: out of memory (10): (no details)
		// this error happens on consumer grade hardware because of nvidia's limit on number of concurrent nvenc limit
		// this is a software limit, see: https://github.com/keylase/nvidia-patch
		return fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	// ok!
	return nil
}
