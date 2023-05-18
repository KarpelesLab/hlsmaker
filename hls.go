package main

import (
	"encoding/binary"
	"fmt"
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
	f     *os.File
	dir   string
	files map[string]int64
}

func newHlsBuilder(out string) (*hlsBuilder, error) {
	file, err := os.Create(out)
	if err != nil {
		return nil, err
	}

	return &hlsBuilder{f: file, files: make(map[string]int64)}, nil
}

func (hls *hlsBuilder) build(in string) error {
	hls.dir = filepath.Dir(in)
	master, err := m3u8Parse(in)
	if err != nil {
		return err
	}
	log.Printf("master = %+v", master)

	var playlists []*m3u8

	for n, f := range master.files {
		pl, err := m3u8Parse(filepath.Join(hls.dir, f.filename))
		if err != nil {
			return err
		}
		playlists = append(playlists, pl)
		f.filename = fmt.Sprintf("%d.m3u8", n)
	}

	// clear output file
	pos := int64(16 + 16 + len(master.files)*16)
	hls.f.Truncate(0)
	hls.f.Seek(pos, io.SeekStart)

	for _, pl := range playlists {
		for _, f := range pl.files {
			pos, err := hls.getFile(f.filename)
			if err != nil {
				return err
			}
			err = f.offsetFile(pos)
			if err != nil {
				return err
			}
			f.filename = "data"
		}
	}

	pos, err = hls.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	hls.writeInt32(8, uint32(len(playlists)))

	var buf []byte
	var ln int

	for n, pl := range playlists {
		buf = pl.Bytes()
		ln, err = hls.f.Write(buf)
		if err != nil {
			return err
		}
		hls.writeInt64(32+(16*n), uint64(pos))
		hls.writeInt32(32+(16*n)+12, uint32(ln))
		pos += int64(ln)
	}

	// write master at the end
	buf = master.Bytes()
	ln, err = hls.f.Write(buf)
	if err != nil {
		return err
	}
	// write info
	hls.writeInt64(16, uint64(pos))
	hls.writeInt32(28, uint32(ln))
	pos += int64(ln)

	// we're all done, now write the header
	hls.f.WriteAt([]byte{'H', 'L', 'S', 1}, 0)

	return nil
}

func (hls *hlsBuilder) getFile(fn string) (int64, error) {
	pos, ok := hls.files[fn]
	if ok {
		return pos, nil
	}
	log.Printf("hls: appending %s", fn)
	full := filepath.Join(hls.dir, fn)
	read, err := os.Open(full)
	if err != nil {
		return 0, err
	}
	defer read.Close()

	pos, err = hls.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// copy data
	_, err = io.Copy(hls.f, read)
	if err != nil {
		return 0, err
	}
	hls.files[fn] = pos
	return pos, nil
}

func (hls *hlsBuilder) writeInt32(pos int, v uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	_, err := hls.f.WriteAt(buf[:], int64(pos))
	return err
}

func (hls *hlsBuilder) writeInt64(pos int, v uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	_, err := hls.f.WriteAt(buf[:], int64(pos))
	return err
}
