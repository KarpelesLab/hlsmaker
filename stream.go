package main

import "fmt"

type hlsStream struct {
	id  int            // stream number
	lid int            // stream number per type
	typ byte           // 'v', 'a' or 's' depending if video/audio/subtitle stream
	src *ffprobeStream // source stream
}

// newStream return a new stream with the correct id set
func (hls *hlsBuilder) newStream(src *ffprobeStream) *hlsStream {
	s := &hlsStream{
		id:  len(hls.streams),
		typ: src.CodecType[0],
		src: src,
	}
	// compute lid
	for _, x := range hls.streams {
		if x.typ == s.typ {
			s.lid += 1
		}
	}
	hls.streams = append(hls.streams, s)
	return s
}

func (s *hlsStream) String() string {
	return fmt.Sprintf("%c:%d", s.typ, s.lid)
}
