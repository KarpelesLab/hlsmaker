package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"
)

// special hls format
// All playlist will reference "data" for data, or <x>.m3u8 for streams with x starting at 0
// master stream is stream -1 and is not addressed directly
//
// Header is 16 bytes:
// HLS<v>
// <4 bytes flags>
// <4 bytes number of streams>
// <4 bytes timestamp>
// for master and each stream: <8 bytes offset of playlist> <4 bytes flags> <4 bytes file length>
//
// total size of header is 16 + 16 + (nstream * 16)

var (
	keepTemp = flag.Bool("keeptemp", false, "keep temporary files")
)

type fileInfo struct {
	pos int64
	ln  int64
}

type hlsBuilder struct {
	f       *os.File
	info    *ffprobeInfo // source file info
	dir     string
	files   map[string]*fileInfo
	streams []*hlsStream
}

const (
	FilePlaylist = iota
	FileMpegTS
	FileMP4
	FileVTT
)

func newHlsBuilder(out string) (*hlsBuilder, error) {
	file, err := os.Create(out)
	if err != nil {
		return nil, err
	}

	// make temp dir
	d, err := os.MkdirTemp("", "hlsmaker*")
	if err != nil {
		return nil, fmt.Errorf("failed to make temp dir: %w", err)
	}
	log.Printf("Using temporary dir: %s", d)

	return &hlsBuilder{f: file, files: make(map[string]*fileInfo), dir: d}, nil
}

func (hls *hlsBuilder) build() error {
	master, err := m3u8Parse(filepath.Join(hls.dir, "master.m3u8"))
	if err != nil {
		return err
	}

	var playlists []*m3u8
	uniqueFiles := make(map[string]int)

	for n, f := range master.files {
		pl, err := m3u8Parse(filepath.Join(hls.dir, f.filename))
		if err != nil {
			return err
		}
		playlists = append(playlists, pl)
		f.filename = fmt.Sprintf("%d.m3u8", n)
		for _, sub := range pl.files {
			uniqueFiles[sub.filename] = 0
		}
	}
	log.Printf("identified %d unique media files", len(uniqueFiles))

	// clear output file
	pos := int64(16 + 16 + (len(master.files)+len(uniqueFiles))*16)
	hls.f.Truncate(0)
	hls.f.Seek(pos, io.SeekStart)

	cnt := len(master.files) // 4

	for _, pl := range playlists {
		for _, f := range pl.files {
			n := uniqueFiles[f.filename]
			pos, ln, err := hls.getFile(f.filename)
			if err != nil {
				return err
			}
			if n == 0 {
				n = cnt
				uniqueFiles[f.filename] = n
				cnt += 1
				hls.writeInt64(32+(16*n), uint64(pos))
				hls.writeInt32(32+(16*n)+8, hlsFlags(f))
				hls.writeInt32(32+(16*n)+12, uint32(ln))
			}
			f.filename = fmt.Sprintf("%d%s", n, path.Ext(f.filename))
		}
	}

	pos, err = hls.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	var buf []byte
	var ln int

	for n, pl := range playlists {
		buf = pl.Bytes()
		ln, err = hls.f.Write(buf)
		if err != nil {
			return err
		}
		hls.writeInt64(32+(16*n), uint64(pos))
		// flags == 0
		hls.writeInt32(32+(16*n)+12, uint32(ln))
		pos += int64(ln)
	}

	// fix master
	err = hls.fixMaster(master)
	if err != nil {
		return fmt.Errorf("failed to fix master: %w", err)
	}

	// write master at the end
	buf = master.Bytes()
	ln, err = hls.f.Write(buf)
	if err != nil {
		return err
	}

	// also write in temp dir just in case (ignore errors)
	os.WriteFile(filepath.Join(hls.dir, "master_fixed.m3u8"), buf, 0644)

	// write info
	hls.writeInt32(8, uint32(cnt))
	hls.writeInt32(12, uint32(time.Now().Unix()))
	hls.writeInt64(16, uint64(pos))
	hls.writeInt32(28, uint32(ln))
	pos += int64(ln)

	// we're all done, now write ID in the header (we do that as final step on purpose)
	hls.f.WriteAt([]byte{'H', 'L', 'S', 1}, 0)

	return nil
}

func (hls *hlsBuilder) getFile(fn string) (int64, int64, error) {
	nfo, ok := hls.files[fn]
	if ok {
		return nfo.pos, nfo.ln, nil
	}
	//log.Printf("hls: appending %s", fn)
	full := filepath.Join(hls.dir, fn)
	read, err := os.Open(full)
	if err != nil {
		return 0, 0, err
	}
	defer read.Close()

	pos, err := hls.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, 0, err
	}

	// copy data
	ln, err := io.Copy(hls.f, read)
	if err != nil {
		return 0, 0, err
	}
	hls.files[fn] = &fileInfo{pos: pos, ln: ln}
	return pos, ln, nil
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

func (hls *hlsBuilder) Close() error {
	if !*keepTemp {
		os.RemoveAll(hls.dir)
	} else {
		log.Printf("Keeping temporary directory for debug, please delete once done: %s", hls.dir)
	}
	return hls.f.Close()
}

func hlsFlags(f *m3u8file) uint32 {
	switch path.Ext(f.filename) {
	case ".m3u8":
		return FilePlaylist
	case ".ts":
		return FileMpegTS
	case ".mp4":
		return FileMP4
	case ".vtt":
		return FileVTT
	default:
		panic(fmt.Sprintf("invalid filename %s", f.filename))
	}
	return 0
}
