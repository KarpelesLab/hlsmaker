package main

import "strconv"

type hlsStream struct {
	id  int            // stream number
	typ string         // "audio", "video" or "subtitle"
	src *ffprobeStream // source stream
}

// newStream return a new invalid stream with the correct id set
func (hls *hlsBuilder) newStream() *hlsStream {
	s := &hlsStream{id: len(hls.streams)}
	hls.streams = append(hls.streams, s)
	return s
}

func (s *hlsStream) String() string {
	return strconv.Itoa(s.id)
}
