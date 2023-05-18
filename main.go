package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
)

var (
	inputFile  = flag.String("in", "", "input file")
	outputFile = flag.String("out", "", "output file")
	encKey     = flag.String("key", "", "encryption key (16 bytes, hexadecimal)")
)

func main() {
	flag.Parse()

	// take input video file (as param to ffmpeg or ffprobe) and generate a video file
	if inputFile == nil || *inputFile == "" {
		log.Printf("Syntax: %s -in filename [-key key]", os.Args[0])
		os.Exit(1)
		return
	}

	// make temp dir
	d, err := os.MkdirTemp("", "hlsmaker*")
	if err != nil {
		log.Printf("failed to make temp dir: %s", err)
		os.Exit(1)
		return
	}
	log.Printf("Using temporary dir: %s", d)
	defer os.RemoveAll(d)

	err = encodeVideo(d) // will generate {d}/master.m3u8
	if err != nil {
		log.Printf("encoding failed: %s", err)
		os.Exit(1)
		return
	}

	out := *outputFile
	if out == "" {
		out = *inputFile + ".hls"
	}

	hlsb, err := newHlsBuilder(out)
	if err != nil {
		log.Printf("failed to create output file: %s", err)
		os.Exit(1)
		return
	}
	err = hlsb.build(filepath.Join(d, "master.m3u8"))
	if err != nil {
		log.Printf("failed to build hls: %s", err)
		os.Exit(1)
		return
	}
}
