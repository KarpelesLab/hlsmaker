package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/KarpelesLab/runutil"
)

type ffprobeInfo struct {
	Streams []*ffprobeStream `json:"streams"`
	Format  *ffprobeFormat   `json:"format"`
}

func ffprobeFile(f string) (*ffprobeInfo, error) {
	var info *ffprobeInfo
	err := runutil.RunJson(&info, exe("ffprobe"), "-print_format", "json", "-hide_banner", "-loglevel", "warning", "-show_format", "-show_streams", "-show_chapters", f)
	return info, err
}

func (info *ffprobeInfo) video() *ffprobeStream {
	return info.stream("video")
}

func (info *ffprobeInfo) audio() *ffprobeStream {
	return info.stream("audio")
}

func (info *ffprobeInfo) stream(typ string) *ffprobeStream {
	for _, s := range info.Streams {
		if s.CodecType == typ {
			return s
		}
	}
	return nil
}

func (info *ffprobeInfo) streams(typ string) []*ffprobeStream {
	var res []*ffprobeStream
	for _, s := range info.Streams {
		if s.CodecType == typ {
			res = append(res, s)
		}
	}
	return res
}

type ffprobeStream struct {
	Index          int         `json:"index"` // stream index
	CodecName      string      `json:"codec_name"`
	CodecLongName  string      `json:"codec_long_name"`
	Profile        string      `json:"profile"`
	CodecType      string      `json:"codec_type"`       // video
	CodecTagString string      `json:"codec_tag_string"` // avc1
	CodecTag       string      `json:"codec_tag"`        // 0x31637661
	Width          int         `json:"width,omitempty"`
	Height         int         `json:"height,omitempty"`
	CodedWidth     int         `json:"coded_width"`
	CodedHeight    int         `json:"coded_height"`
	ClosedCaptions int         `json:"closed_captions"`
	FrameRate      ffFrameRate `json:"r_frame_rate"`
	SampleRate     int         `json:"sample_rate,string"`
	Duration       float64     `json:"duration,string"`
	StartTime      float64     `json:"start_time,string"`
	StartPTS       int         `json:"start_pts"`

	Disposition *struct {
		Default int `json:"default"`
		Dub     int `json:"dub"`
	} `json:"disposition,omitempty"`

	Tags map[string]string `json:"tags"`
}

type ffprobeFormat struct {
	Filename       string            `json:"filename"`
	FormatName     string            `json:"format_name"`       // mov,mp4,m4a,3gp,3g2,mj2
	FormatLongName string            `json:"format_long_name"`  // QuickTime / MOV
	StartTime      float64           `json:"start_time,string"` // "start_time": "0.000000",
	Duration       float64           `json:"duration,string"`   // "duration": "240.048000",
	Size           int64             `json:"size,string"`       // "size": "73087904",
	BitRate        int               `json:"bit_rate,string"`   // "bit_rate": "2435776",
	Tags           map[string]string `json:"tags"`
}

type ffFrameRate string // typically: 25/1 or 30000/1001

func (f ffFrameRate) Value() float64 {
	p := strings.IndexByte(string(f), '/')
	if p == -1 {
		// parse as float
		v, err := strconv.ParseFloat(string(f), 64)
		if err != nil {
			return math.NaN()
		}
		return v
	}
	a := string(f[:p])   // 25
	b := string(f[p+1:]) // 1

	ai, err := strconv.ParseUint(a, 10, 64)
	if err != nil {
		return math.NaN()
	}
	bi, err := strconv.ParseUint(b, 10, 64)
	if err != nil {
		return math.NaN()
	}

	return float64(ai) / float64(bi)
}
