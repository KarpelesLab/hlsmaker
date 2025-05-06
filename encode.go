package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/KarpelesLab/ffprobe"
	"github.com/KarpelesLab/runutil"
)

var (
	maxStreams   = flag.Int("max_streams", 16, "set the maximum number of video streams to include")
	softwareMode = flag.Bool("software", false, "enable software encoding")
	softFallback = flag.Bool("software_fallback", false, "enable software encoding if hardware fails")
	singleMode   = flag.Bool("single", false, "do not encode video, copy only")
	verboseMode  = flag.Bool("verbose", false, "show more info during encoding")
	videoFilters = flag.String("filter_complex", "", "add extra video filters such as yadif in ffmpeg format")
)

func (hls *hlsBuilder) prepareVideo(input string) error {
	hls.input = input
	// perform ffprobe
	err := runutil.RunJson(&hls.info, exe("ffprobe"), "-print_format", "json", "-hide_banner", "-loglevel", "warning", "-show_format", "-show_streams", "-show_chapters", input)
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	hls.video = hls.info.Video()
	hls.audios = hls.info.GetStreams("audio")
	hls.subtitles = hls.info.GetStreams("subtitle")
	if hls.video == nil {
		return fmt.Errorf("video track missing")
	}

	log.Printf("input: video stream format %s %dx%d", hls.video.CodecName, hls.video.Width, hls.video.Height)
	for _, audio := range hls.audios {
		lng, ok := audio.Tags["language"]
		if ok {
			log.Printf("input: audio format %s %d Hz, language %s", audio.CodecName, audio.SampleRate, lng)
		} else {
			log.Printf("input: audio format %s %d Hz", audio.CodecName, audio.SampleRate)
		}
	}
	var usableSubs []*ffprobe.Stream
	for _, subtitle := range hls.subtitles {
		switch subtitle.CodecName {
		case "dvd_subtitle", "subrip", "hdmv_pgs_subtitle":
			// format is not usabe
			// "Error initializing output stream 0:0 -- Subtitle encoding currently only possible from text to text or bitmap to bitmap"
		default:
			usableSubs = append(usableSubs, subtitle)
		}
		lng, ok := subtitle.Tags["language"]
		if !ok {
			lng = "und"
		}
		log.Printf("input: subtitles format %s language %s", subtitle.CodecName, lng)
	}
	hls.subtitles = usableSubs

	siz := &vsize{w: hls.video.Width, h: hls.video.Height}

	// generate variant sizes
	hls.variants = nil
	if *singleMode {
		// this mode is mostly used for testing
		hls.variants = append(hls.variants, &hlsVariant{size: siz, codec: H264})
		log.Printf("will be generating the following sizes (single mode enabled): %v", hls.variants)
		return nil
	}
	hls.variants = append(hls.variants, siz.variants()...)
	for siz = siz.smaller(); siz != nil; siz = siz.smaller() {
		if len(hls.variants) >= *maxStreams {
			break
		}
		hls.variants = append(hls.variants, siz.variants()...)
	}

	log.Printf("will be generating the following sizes: %v", hls.variants)
	return nil
}

func (hls *hlsBuilder) encodeVideo() error {
	// prepare the command line
	args := []string{"-hide_banner"}

	// reset stuff
	hls.streams = nil

	if !*verboseMode {
		args = append(args, "-loglevel", "warning")
	}
	if !*softwareMode {
		args = append(args, "-hwaccel", "auto")
	}

	args = append(args, "-i", hls.input)

	softwareEncode := *softwareMode

	// prepare filter_complex
	var flt string
	if videoFilters != nil && *videoFilters != "" {
		flt = fmt.Sprintf("[v:0]%s,split=%d", *videoFilters, len(hls.variants))
	} else {
		flt = fmt.Sprintf("[v:0]split=%d", len(hls.variants))
	}
	for n := range hls.variants {
		if n == 0 {
			flt += "[v0]"
			continue
		}
		flt += fmt.Sprintf("[vin%d]", n)
	}
	for n, s := range hls.variants {
		if n == 0 {
			// n==0 means original size
			continue
		}
		flt += fmt.Sprintf(";[vin%d]%s[v%d]", n, s.size.Scale(), n)
	}
	args = append(args, "-filter_complex", flt)

	// map filters
	rate := hls.video.FrameRate.Value()

	// force good framerate values
	if rate > 60 {
		rate = 60
	} else if rate < 10 {
		rate = 10
	}

	for n, v := range hls.variants {
		codec := v.codec
		ns := strconv.Itoa(n)
		ts := hls.newStream(hls.video)

		args = append(args, "-map", "[v"+ns+"]")
		args = append(args, codec.Args(softwareEncode, rate, v.size).Expand()...)

		args = append(args, ts.Filename())
	}

	// audio
	for n, audio := range hls.audios {
		ns := strconv.Itoa(n)
		ts := hls.newStream(audio)
		args = append(args,
			"-map", "a:"+ns,
			"-c", "aac",
			"-b:a", "96k",
			"-ac", "2",
		)
		args = append(args, ts.Filename())
	}

	if *verboseMode {
		log.Printf("ffmpeg arguments: %v", args)
	}
	c := exec.Command(exe("ffmpeg"), args...)
	c.Dir = hls.dir // set to run in temp dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Run()
	if err != nil {
		// [h264_nvenc @ 0x558dde66e480] OpenEncodeSessionEx failed: out of memory (10): (no details)
		// this error happens on consumer grade hardware because of nvidia's limit on number of concurrent nvenc limit
		// this is a software limit, see: https://github.com/keylase/nvidia-patch
		return fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	if len(hls.subtitles) == 0 {
		// no subtitles, process ends here
		return nil
	}

	// Check if stream_0.mp4 exists before proceeding with subtitle extraction
	streamPath := filepath.Join(hls.dir, "stream_0.mp4")
	if _, err := os.Stat(streamPath); os.IsNotExist(err) {
		return fmt.Errorf("stream_0.mp4 not found, video encoding may have failed")
	}

	// fetch StartPTS for first video
	s0, err := ffprobe.Probe(filepath.Join(hls.dir, "stream_0.mp4"))
	if err != nil {
		return fmt.Errorf("failed to probe stream_0.mp4: %w", err)
	}

	video := s0.Video()
	if video == nil {
		return fmt.Errorf("no video stream found in stream_0.mp4")
	}
	startTime := video.StartTime

	// extract subtitles one by one
	for n, subtitle := range hls.subtitles {
		// prepare the command line
		args = []string{"-itsoffset", strconv.FormatFloat(startTime, 'f', -1, 64), "-i", hls.input, "-hide_banner"}

		if !*verboseMode {
			args = append(args, "-loglevel", "warning")
		}

		ns := strconv.Itoa(n)
		ts := hls.newStream(subtitle)
		args = append(args,
			"-map", "s:"+ns,
			"-c:0", "webvtt",
			"-f", "webvtt",
			// output file
			ts.Filename(),
		)
		if *verboseMode {
			log.Printf("ffmpeg arguments: %v", args)
		}
		c := exec.Command(exe("ffmpeg"), args...)
		c.Dir = hls.dir // set to run in temp dir
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		err = c.Run()
		if err != nil {
			return fmt.Errorf("failed to run ffmpeg for %s: %w", ts, err)
		}

		// generate stream file
		hls.makeSubPlaylist(ts)
	}

	// ok!
	return nil
}

func (hls *hlsBuilder) makeSubPlaylist(ts *hlsStream) error {
	s := ts.src

	res := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:6",
		"#EXT-X-ALLOW-CACHE:YES",
		fmt.Sprintf("#EXT-X-TARGETDURATION:%.0f", s.Duration),
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		fmt.Sprintf("#EXTINF:%.06f,", s.Duration),
		fmt.Sprintf("subs_%d.vtt", ts.id),
		"#EXT-X-ENDLIST",
	}

	// write file
	fn := filepath.Join(hls.dir, fmt.Sprintf("%d.m3u8", ts.id))
	err := os.WriteFile(fn, []byte(strings.Join(res, "\n")+"\n"), 0644)
	if err != nil {
		return err
	}

	// append to master file
	f, err := os.OpenFile(filepath.Join(hls.dir, "master.m3u8"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := []string{
		"#EXT-X-STREAM-INF:BANDWIDTH=0,CODECS=\"webvtt\"",
		fmt.Sprintf("%d.m3u8", ts.id),
	}

	_, err = f.Write([]byte(strings.Join(buf, "\n") + "\n"))
	return err
}
