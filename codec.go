package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strconv"
)

type Codec int

// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -encoders
const (
	H264 Codec = iota
	HEVC
	AV1
)

var (
	codecTags    = map[string]string{"hevc_nvenc": "hvc1", "av1_nvenc": "av01"}
	codecProfile = map[string]string{"h264_nvenc": "main", "hevc_nvenc": "main"}
	fastEncode   = flag.Bool("fast_encode", false, "enable fast encoding with lower quality")
)

func (c Codec) nvencName() string {
	switch c {
	case H264:
		return "h264_nvenc"
	case HEVC:
		return "hevc_nvenc"
	case AV1:
		return "av1_nvenc"
	default:
		return "bad_nvenc"
	}
}

func (c Codec) String() string {
	switch c {
	case H264:
		return "h264"
	case HEVC:
		return "hevc"
	case AV1:
		return "av1"
	default:
		return "invalid codec"
	}
}

func (c Codec) codecPreset(software bool) string {
	if software {
		if *fastEncode {
			return "ultrafast"
		} else {
			return "slow"
		}
	}
	if *fastEncode {
		return "p1"
	} else {
		// p6 = nvenc: slower (better quality)
		return "p6"
	}
}

func (c Codec) Args(software bool, tsid string, rate float64, s *vsize) []string {
	bitrateInt := s.bitrate(rate, 0.1) // TODO make 0.1 depend on cookie
	br := strconv.FormatUint(bitrateInt, 10)

	if !software {
		// ensure this codec can be used
		if err := c.testHardware(s); err != nil {
			log.Printf("Using software encoding for codec %s as hardware encoding failed: %s", err)
			software = true
		}
	}

	if !software {
		codec := c.nvencName()
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=av1_nvenc

		res := []string{
			"-c:" + tsid, codec,
			"-pix_fmt:" + tsid, "yuv420p",
			"-preset:" + tsid, c.codecPreset(software),
			"-b:" + tsid, br,
			"-maxrate:" + tsid, br,
		}
		if prof, ok := codecProfile[codec]; ok {
			res = append(res, "-profile:"+tsid, prof)
		}
		if tag, ok := codecTags[codec]; ok {
			res = append(res, "-tag:"+tsid, tag)
		}
		return res
	}

	switch c {
	case H264:
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=libx264
		res := []string{
			"-c:" + tsid, "libx264",
			"-x264-params", "nal-hrd=cbr:force-cfr=1",
			"-b:" + tsid, br,
			"-maxrate:" + tsid, br,
			"-minrate:" + tsid, br,
			"-bufsize:" + tsid, strconv.FormatUint(bitrateInt*2, 10),
			"-preset:" + tsid, c.codecPreset(software),
			"-g:" + tsid, "48",
			"-sc_threshold:" + tsid, "0",
			"-keyint_min:" + tsid, "48",
		}
		return res
	case HEVC:
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=libx265
		res := []string{
			"-c:" + tsid, "libx265",
			"-b:" + tsid, br,
			"-maxrate:" + tsid, br,
			"-minrate:" + tsid, br,
			"-bufsize:" + tsid, strconv.FormatUint(bitrateInt*2, 10),
			"-tag:" + tsid, "hvc1",
			"-preset:" + tsid, c.codecPreset(software),
		}
		return res
	case AV1:
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=libaom-av1
		res := []string{
			"-c:" + tsid, "libaom-av1",
			"-b:" + tsid, br,
			"-maxrate:" + tsid, br,
			"-minrate:" + tsid, br,
			"-bufsize", strconv.FormatUint(bitrateInt*2, 10),
			"-preset:" + tsid, c.codecPreset(software),
		}
		return res
	default:
		return []string{"error", "unsupported_codec"}
	}
}

func (codec Codec) testHardware(size *vsize) error {
	// some codecs such as h264_nvenc may not support some encoding sizes (4k or 8k) or appear available but not actually work
	// this will attempt to encode a single frame using the provided codec & size and report any error
	//
	// some errors we catch this way:
	// [hevc_nvenc @ 0x55dc5d8542c0] Driver does not support the required nvenc API version. Required: 12.1 Found: 12.0
	// [h264_nvenc @ 0x560455c88540] No capable devices found
	// Segmentation fault (core dumped)
	c := exec.Command(exe("ffmpeg"), "-loglevel", "error", "-f", "lavfi", "-i", "color=black:s="+size.String(), "-vframes", "1", "-an", "-c:v", codec.nvencName(), "-f", "null", "-")
	c.Dir = os.TempDir()
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c.Run()
}
