package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
)

var (
	inputFile  = flag.String("in", "", "input file (required)")
	outputFile = flag.String("out", "", "output file")
)

func main() {
	flag.Parse()

	// take input video file (as param to ffmpeg or ffprobe) and generate a video file
	if inputFile == nil || *inputFile == "" {
		log.Printf("Syntax: %s -in filename [-key key]", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
		return
	}

	inFile, err := filepath.Abs(*inputFile)
	if err != nil {
		log.Printf("Invalid input path: %s", err)
		os.Exit(1)
		return
	}
	if _, err = os.Stat(inFile); err != nil {
		log.Printf("Input file could not be located: %s", err)
		os.Exit(1)
		return
	}

	out := *outputFile
	if out == "" {
		out = inFile + ".hls"
	}

	hlsb, err := newHlsBuilder(out)
	if err != nil {
		log.Printf("failed to create output file: %s", err)
		os.Exit(1)
		return
	}
	defer hlsb.Close()

	err = hlsb.encodeVideo(inFile) // will generate {hls.dir}/master.m3u8
	if err != nil {
		log.Printf("encoding failed: %s", err)
		os.Exit(1)
		return
	}

	err = hlsb.build()
	if err != nil {
		log.Printf("failed to build hls: %s", err)
		os.Exit(1)
		return
	}
}
