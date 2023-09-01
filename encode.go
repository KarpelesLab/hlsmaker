package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/KarpelesLab/runutil"
)

var (
	maxStreams   = flag.Int("max_streams", 16, "set the maximum number of video streams to include")
	softwareMode = flag.Bool("software", false, "enable software encoding")
	encKey       = flag.String("key", "", "encryption key (16 bytes, hexadecimal)")
	singleFile   = flag.Bool("single_file", false, "enable single file mode")
	verboseMode  = flag.Bool("verbose", false, "show more info during encoding")
	videoFilters = flag.String("filter_complex", "", "add extra video filters such as yadif in ffmpeg format")
)

func (hls *hlsBuilder) encodeVideo(input string) error {
	// perform ffprobe
	err := runutil.RunJson(&hls.info, exe("ffprobe"), "-print_format", "json", "-hide_banner", "-loglevel", "warning", "-show_format", "-show_streams", "-show_chapters", input)
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	video := hls.info.video()
	audios := hls.info.streams("audio")
	subtitles := hls.info.streams("subtitle")
	if video == nil {
		return fmt.Errorf("video track missing")
	}

	log.Printf("input: video stream format %s %dx%d", video.CodecName, video.Width, video.Height)
	for _, audio := range audios {
		lng, ok := audio.Tags["language"]
		if ok {
			log.Printf("input: audio format %s %d Hz, language %s", audio.CodecName, audio.SampleRate, lng)
		} else {
			log.Printf("input: audio format %s %d Hz", audio.CodecName, audio.SampleRate)
		}
	}
	var usableSubs []*ffprobeStream
	for _, subtitle := range subtitles {
		switch subtitle.CodecName {
		case "dvd_subtitle", "hdmv_pgs_subtitle":
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
	subtitles = usableSubs

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
	args := []string{"-i", input, "-hide_banner"}

	if !*verboseMode {
		args = append(args, "-loglevel", "warning")
	}

	// prepare filter_complex
	var flt string
	if videoFilters != nil && *videoFilters != "" {
		flt = fmt.Sprintf("[v:0]%s,split=%d", *videoFilters, len(variants))
	} else {
		flt = fmt.Sprintf("[v:0]split=%d", len(variants))
	}
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

	var varStreamMap []string
	for n, s := range variants {
		ns := strconv.Itoa(n)
		ts := hls.newStream(video)
		tsid := ts.String()
		bitrateInt := s.bitrate(rate, 0.1)
		br := strconv.FormatUint(bitrateInt, 10) // we use 0.1 bit per pixel for now
		if *softwareMode {
			args = append(args,
				"-map", "[v"+ns+"]",
				"-c:"+tsid, "libx264",
				"-x264-params", "nal-hrd=cbr:force-cfr=1",
				"-b:"+tsid, br,
				"-maxrate:"+tsid, br,
				"-minrate:"+tsid, br,
				"-bufsize", strconv.FormatUint(bitrateInt*2, 10),
				"-preset", "slow",
				"-g", "48",
				"-sc_threshold", "0",
				"-keyint_min", "48",
			)
		} else {
			args = append(args,
				"-map", fmt.Sprintf("[v%d]", n),
				"-c:"+tsid, "h264_nvenc",
				"-profile:"+tsid, "high",
				"-pix_fmt:"+tsid, "yuv420p",
				"-preset:"+tsid, "p5",
				"-b:"+tsid, br,
				"-maxrate:"+tsid, br,
			)
		}
		varStreamMap = append(varStreamMap, tsid)
	}

	// audio
	for n, audio := range audios {
		ns := strconv.Itoa(n)
		ts := hls.newStream(audio)
		tsid := ts.String()
		args = append(args,
			"-map", "a:"+ns,
			"-c:"+tsid, "aac",
			"-b:"+tsid, "96k",
			"-ac:"+tsid, "2",
		)
		varStreamMap = append(varStreamMap, tsid)
	}

	hlsFlags := []string{"independent_segments"}
	if *singleFile {
		hlsFlags = append(hlsFlags, "single_file")
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "10",
		"-hls_playlist_type", "vod",
		"-hls_flags", strings.Join(hlsFlags, "+"),
		"-hls_allow_cache", "1",
		"-hls_segment_type", "mpegts",
		"-master_pl_name", "master.m3u8",
	)
	if *singleFile {
		args = append(args, "-hls_segment_filename", "stream_%v.ts")
	} else {
		args = append(args, "-hls_segment_filename", "stream_%v_%d.ts")
	}

	if encKey != nil && *encKey != "" {
		key, err := hex.DecodeString(*encKey)
		if err != nil {
			return fmt.Errorf("could not decode encryption key: %w", err)
		}
		iv := make([]byte, 16)
		_, err = io.ReadFull(rand.Reader, iv)
		if err != nil {
			return fmt.Errorf("could not read random data for IV: %w", err)
		}
		// write key to disk
		os.WriteFile(filepath.Join(hls.dir, "master.key"), key, 0600)
		// generate key info file
		os.WriteFile(filepath.Join(hls.dir, "keyinfo.txt"), []byte("master.key\n"+filepath.Join(hls.dir, "master.key")+"\n"+hex.EncodeToString(iv)+"\n"), 0600)
		args = append(args,
			"-hls_key_info_file", filepath.Join(hls.dir, "keyinfo.txt"),
		)
	}

	args = append(args,
		"-var_stream_map", strings.Join(varStreamMap, " "),
		"stream_%v.m3u8",
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
		// [h264_nvenc @ 0x558dde66e480] OpenEncodeSessionEx failed: out of memory (10): (no details)
		// this error happens on consumer grade hardware because of nvidia's limit on number of concurrent nvenc limit
		// this is a software limit, see: https://github.com/keylase/nvidia-patch
		return fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	if len(subtitles) == 0 {
		// no subtitles, process ends here
		return nil
	}

	// fetch StartPTS for stream_0_0.ts
	s0, err := ffprobeFile(filepath.Join(hls.dir, "stream_0_0.ts"))
	if err != nil {
		return err
	}
	startTime := s0.video().StartTime

	// extract subtitles one by one
	for n, subtitle := range subtitles {
		// prepare the command line
		args = []string{"-itsoffset", strconv.FormatFloat(startTime, 'f', -1, 64), "-i", input, "-hide_banner"}

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
			fmt.Sprintf("subs_%d.vtt", ts.id),
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
