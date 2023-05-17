package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type m3u8 struct {
	headers []string
	files   []*m3u8file
	footer  []string
}

type m3u8file struct {
	headers  []string
	filename string
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

		if strings.HasPrefix(ln, "#EXT-X-STREAM-INF:") || strings.HasPrefix(ln, "#EXTINF:") {
			// we're now in a file
			if f != nil {
				return fmt.Errorf("unexpected %s", ln)
			}
			f = &m3u8file{
				headers: []string{ln},
			}
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
			m.files = append(m.files, f)
			f = nil
			continue
		}
		if f == nil {
			if len(m.files) != 0 {
				return fmt.Errorf("unexpected %s", ln)
			}
			m.headers = append(m.headers, ln)
		} else {
			f.headers = append(f.headers, ln)
		}
	}
}

func (m *m3u8) WriteTo(w io.Writer) (n int64, err error) {
	var n2 int
	var n3 int64
	for _, h := range m.headers {
		n2, err = w.Write([]byte(h + "\n"))
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
		n2, err = w.Write([]byte(h + "\n"))
		n += int64(n2)
		if err != nil {
			return
		}
	}
	n2, err = w.Write([]byte(f.filename + "\n"))
	n += int64(n2)
	return
}
