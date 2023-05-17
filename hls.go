package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// special hls format
// All playlist will reference "data" for data, or <x>.m3u8 for streams with x starting at 0
// master stream is stream -1 and is not addressed directly
//
// Header is 16 bytes:
// HLS<v>
// <4 bytes flags>
// <4 bytes number of streams>
// <4 bytes unused>
// for master and each stream: <8 bytes offset of playlist> <4 bytes flags> <4 bytes file length>
//
// total size of header is 16 + 16 + (nstream * 16)

type hlsBuilder struct {
	f *os.File
}

func newHlsBuilder(out string) (*hlsBuilder, error) {
	file, err := os.Create(out)
	if err != nil {
		return nil, err
	}

	return &hlsBuilder{f: file}, nil
}

func (hls *hlsBuilder) build(in string) error {
	dir := filepath.Dir(in)
	master, err := m3u8Parse(in)
	if err != nil {
		return err
	}
	log.Printf("master = %+v", master)

	var playlists []*m3u8

	for _, f := range master.files {
		pl, err := m3u8Parse(filepath.Join(dir, f.filename))
		if err != nil {
			return err
		}
		playlists = append(playlists, pl)
	}

	// clear output file
	pos := int64(16 + 16 + len(master.files)*16)
	hls.f.Truncate(0)
	hls.f.Seek(pos, io.SeekStart)

	return nil
}
