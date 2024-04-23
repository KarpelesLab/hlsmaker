package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strconv"
)

type Codec int
type CodecArgs []*codecArg

type codecArg struct {
	K, V string // key, value
}

func (args CodecArgs) WithTsid(tsid string) []string {
	res := make([]string, 0, len(args)*2)
	for _, a := range args {
		res = append(res, a.K+":"+tsid, a.V)
	}
	return res
}

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

func (c Codec) Args(software bool, rate float64, s *vsize) CodecArgs {
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

		res := CodecArgs{
			&codecArg{"-c", codec},
			&codecArg{"-pix_fmt", "yuv420p"},
			&codecArg{"-preset", c.codecPreset(software)},
			&codecArg{"-b", br},
			&codecArg{"-maxrate", br},
		}
		if prof, ok := codecProfile[codec]; ok {
			res = append(res, &codecArg{"-profile", prof})
		}
		if tag, ok := codecTags[codec]; ok {
			res = append(res, &codecArg{"-tag", tag})
		}
		return res
	}

	switch c {
	case H264:
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=libx264
		res := CodecArgs{
			&codecArg{"-c", "libx264"},
			&codecArg{"-x264-params", "nal-hrd=cbr:force-cfr=1"},
			&codecArg{"-b", br},
			&codecArg{"-maxrate", br},
			&codecArg{"-minrate", br},
			&codecArg{"-bufsize", strconv.FormatUint(bitrateInt*2, 10)},
			&codecArg{"-preset", c.codecPreset(software)},
			&codecArg{"-g", "48"},
			&codecArg{"-sc_threshold", "0"},
			&codecArg{"-keyint_min", "48"},
		}
		return res
	case HEVC:
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=libx265
		res := CodecArgs{
			&codecArg{"-c", "libx265"},
			&codecArg{"-b", br},
			&codecArg{"-maxrate", br},
			&codecArg{"-minrate", br},
			&codecArg{"-bufsize", strconv.FormatUint(bitrateInt*2, 10)},
			&codecArg{"-tag", "hvc1"},
			&codecArg{"-preset", c.codecPreset(software)},
		}
		return res
	case AV1:
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=libaom-av1
		res := CodecArgs{
			&codecArg{"-c", "libaom-av1"},
			&codecArg{"-b", br},
			&codecArg{"-maxrate", br},
			&codecArg{"-minrate", br},
			&codecArg{"-bufsize", strconv.FormatUint(bitrateInt*2, 10)},
			&codecArg{"-preset", c.codecPreset(software)},
		}
		return res
	default:
		return nil
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
