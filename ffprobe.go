package main

type ffprobeInfo struct {
	Streams []*ffprobeStream `json:"streams"`
	Format  *ffprobeFormat   `json:"format"`
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

type ffprobeStream struct {
	Index          int    `json:"index"` // stream index
	CodecName      string `json:"codec_name"`
	CodecLongName  string `json:"codec_long_name"`
	Profile        string `json:"profile"`
	CodecType      string `json:"codec_type"`       // video
	CodecTagString string `json:"codec_tag_string"` // avc1
	CodecTag       string `json:"codec_tag"`        // 0x31637661
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	CodedWidth     int    `json:"coded_width"`
	CodedHeight    int    `json:"coded_height"`
	ClosedCaptions int    `json:"closed_captions"`
	SampleRate     int    `json:"sample_rate,string"`

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
