package main

import "strconv"

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

func (c Codec) Args(software bool, tsid string, rate float64, s *vsize) []string {
	bitrateInt := s.bitrate(rate, 0.1) // TODO make 0.1 depend on cookie
	br := strconv.FormatUint(bitrateInt, 10)

	if !software && c == H264 && s.isOver(2048) {
		// can't use h264 nvenc with anything more than 1080p ?
		software = true
	}

	if !software {
		codec := c.nvencName()
		// /pkg/main/media-video.ffmpeg.core/bin/ffmpeg -h encoder=av1_nvenc

		res := []string{
			"-c:" + tsid, codec,
			"-pix_fmt:" + tsid, "yuv420p",
			"-preset:" + tsid, "p6", // nvenc: slower (better quality)
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
		res := []string{
			"-c:" + tsid, "libx264",
			"-x264-params", "nal-hrd=cbr:force-cfr=1",
			"-b:" + tsid, br,
			"-maxrate:" + tsid, br,
			"-minrate:" + tsid, br,
			"-bufsize", strconv.FormatUint(bitrateInt*2, 10),
			"-preset", "slow",
			"-g", "48",
			"-sc_threshold", "0",
			"-keyint_min", "48",
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
			"-preset:" + tsid, "slow",
		}
		return res
	default:
		return []string{"error", "unsupported_codec"}
	}
}
