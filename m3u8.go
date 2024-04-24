package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strings"
)

type m3u8 struct {
	headers []*m3u8spec
	files   []*m3u8file
	footer  []string
}

type m3u8file struct {
	headers    []*m3u8spec
	standalone bool
	filename   string
}

type m3u8spec struct {
	key  string   // EXT-X-MEDIA
	vars []string // TYPE=AUDIO, URI="...", etc
}

func m3u8Parse(fn string) (*m3u8, error) {
	log.Printf("m3u8: parsing %s", fn)

	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res := &m3u8{}
	return res, res.parse(f)
}

func (m *m3u8) parse(in io.Reader) error {
	r := bufio.NewReader(in)
	var f *m3u8file

	for {
		ln, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if f != nil {
					return io.ErrUnexpectedEOF
				}
				return nil
			}
		}
		ln = strings.TrimSpace(ln)
		if ln == "" {
			// remove empty lines
			continue
		}
		spec := m3u8specParse(ln)

		if spec.key == "#EXT-X-STREAM-INF" || spec.key == "#EXTINF" {
			// we're now in a file
			if f != nil {
				return fmt.Errorf("unexpected %s", ln)
			}
			f = &m3u8file{
				headers: []*m3u8spec{spec},
			}
			continue
		}
		if spec.key == "#EXT-X-MEDIA" || spec.key == "#EXT-X-I-FRAME-STREAM-INF" {
			// #EXT-X-MEDIA:TYPE=AUDIO,URI="stream_2.m3u8",GROUP-ID="default-audio-group",LANGUAGE="ja",NAME="stream_2",DEFAULT=NO,AUTOSELECT=YES,CHANNELS="2"
			// #EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=321394,AVERAGE-BANDWIDTH=115404,CODECS="avc1.42c028",RESOLUTION=1920x1080,CLOSED-CAPTIONS=NONE,URI="stream_0_iframe.m3u8"
			f := &m3u8file{
				headers:  []*m3u8spec{spec},
				filename: spec.get("URI"),
			}
			m.files = append(m.files, f)
			continue
		}
		if ln == "#EXT-X-ENDLIST" {
			m.footer = append(m.footer, ln)
			continue
		}
		if ln[0] != '#' {
			// filename
			if f == nil {
				return fmt.Errorf("unexpected %s", ln)
			}
			f.filename = ln
			f.standalone = true
			m.files = append(m.files, f)
			f = nil
			continue
		}
		if f == nil {
			if len(m.files) != 0 {
				return fmt.Errorf("unexpected %s", ln)
			}
			m.headers = append(m.headers, spec)
		} else {
			f.headers = append(f.headers, spec)
		}
	}
}

func (m *m3u8) Bytes() []byte {
	buf := &bytes.Buffer{}
	m.WriteTo(buf)
	return buf.Bytes()
}

func (m *m3u8) takeFile(fn string) (f *m3u8file, err error) {
	for n, f := range m.files {
		if f.filename == fn {
			// remove it from files
			m.files = append(m.files[:n], m.files[n+1:]...)
			return f, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (m *m3u8) WriteTo(w io.Writer) (n int64, err error) {
	var n2 int
	var n3 int64
	for _, h := range m.headers {
		n2, err = w.Write([]byte(h.String() + "\n"))
		n += int64(n2)
		if err != nil {
			return
		}
	}
	for _, f := range m.files {
		n3, err = f.WriteTo(w)
		n += n3
		if err != nil {
			return
		}
	}
	for _, ln := range m.footer {
		n2, err = w.Write([]byte(ln + "\n"))
		n += int64(n2)
		if err != nil {
			return
		}
	}
	return
}

func (f *m3u8file) WriteTo(w io.Writer) (n int64, err error) {
	var n2 int
	for _, h := range f.headers {
		n2, err = w.Write([]byte(h.String() + "\n"))
		n += int64(n2)
		if err != nil {
			return
		}
	}
	if f.standalone {
		n2, err = w.Write([]byte(f.filename + "\n"))
		n += int64(n2)
	}
	return
}

func (f *m3u8file) setFilename(fn string) {
	f.filename = fn
	if !f.standalone {
		// header 0 ?
		f.headers[0].set("URI", "\""+fn+"\"")
	}
}

func m3u8specParse(f string) *m3u8spec {
	// #AAA:X=A,Y=B,Z=C

	pos := strings.IndexByte(f, ':')
	if pos == -1 {
		return &m3u8spec{key: f}
	}

	res := &m3u8spec{key: f[:pos]}
	f = f[pos+1:]

	// TODO handle quotes "..."

	for {
		pos = strings.IndexByte(f, ',')
		if pos == -1 {
			// last
			if f != "" {
				res.vars = append(res.vars, f)
			}
			break
		}
		res.vars = append(res.vars, f[:pos])
		f = f[pos+1:]
	}

	return res
}

func (spec *m3u8spec) get(k string) string {
	pfx := k + "="
	for _, s := range spec.vars {
		if strings.HasPrefix(s, pfx) {
			res := s[len(pfx):]
			if res[0] == '"' {
				res = strings.Trim(res, "\"")
			}
			return res
		}
	}
	return ""
}

func (spec *m3u8spec) set(k, v string) {
	pfx := k + "="
	for n, s := range spec.vars {
		if strings.HasPrefix(s, pfx) {
			spec.vars[n] = pfx + v
			return
		}
	}
}

func (spec *m3u8spec) String() string {
	buf := &bytes.Buffer{}
	buf.WriteString(spec.key)
	if len(spec.vars) == 0 {
		return buf.String()
	}

	buf.WriteByte(':')

	for n, v := range spec.vars {
		if n != 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(v)
	}
	return buf.String()
}
