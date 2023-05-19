package main

import (
	"fmt"
	"strings"
)

func (hls *hlsBuilder) fixMaster(master *m3u8) error {
	// master file needs a number of fixes from what ffmpeg does

	// Audio streams as stored by FFMPEG:
	// #EXT-X-STREAM-INF:BANDWIDTH=105600,CODECS="mp4a.40.2"
	// stream_3.m3u8
	//
	// https://developer.apple.com/documentation/http_live_streaming/example_playlists_for_http_live_streaming/adding_alternate_media_to_a_playlist
	//
	// How we want to to actually be:
	// #EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="stereo",LANGUAGE="en",NAME="English",DEFAULT=YES,AUTOSELECT=YES,URI="audio/stereo/en/128kbit.m3u8"
	//
	// We can include CHANNELS=2 (or 5, etc) to specify if this is a multi-channel audio or not
	//
	// TYPE can be AUDIO, VIDEO, SUBTITLES, CLOSED-CAPTIONS
	//
	// LANGUAGE and NAME should be taken from the metadata of the source file. NAME is REQUIRED. (use tags["title"] if found, or "Audio track")
	//
	// GROUP-ID can be "audio" so we can specify it easily to the audio tracks
	// LANGUAGE="dubbing" exists
	//
	// It looks like we lose data such as BANDIWDTH and CODECS however, but maybe these can be put into an intermediate m3u8 file?
	//
	// Other type:
	// #EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="Deutsch",DEFAULT=NO,AUTOSELECT=YES,FORCED=NO,LANGUAGE="de",URI="subtitles_de.m3u8"
	//

	// we also need to modify the existing video files to add:
	// #EXT-X-STREAM-INF:BANDWIDTH=1100000,RESOLUTION=640x360,CODECS="avc1.64001e" ,AUDIO="audio"

	first := true
	hasAudio := false

	for _, stream := range hls.streams {
		if stream.typ == AudioStream {
			f, err := master.takeFile(fmt.Sprintf("%d.m3u8", stream.id))
			if err != nil {
				return fmt.Errorf("could not remove stream %d: %w", stream.id, err)
			}

			props := []string{
				"TYPE=AUDIO",
				"GROUP-ID=\"audio\"",
			}
			if nam, ok := stream.src.Tags["title"]; ok {
				props = append(props, fmt.Sprintf("NAME=\"%s\"", nam))
			} else {
				props = append(props, "NAME=\"Undefined audio track\"")
			}
			if lng, ok := stream.src.Tags["language"]; ok {
				props = append(props, fmt.Sprintf("LANGUAGE=\"%s\"", lng))
			}
			if first {
				props = append(props, "DEFAULT=YES")
				first = false
			} else {
				props = append(props, "DEFAULT=NO")
			}
			props = append(props, "AUTOSELECT=YES")
			props = append(props, fmt.Sprintf("URI=\"%s\"", f.filename))

			final := "#EXT-X-MEDIA:" + strings.Join(props, ",")
			master.headers = append(master.headers, final)
			hasAudio = true
		}
	}

	if hasAudio {
		for _, f := range master.files {
			f.headers[0] += ",AUDIO=\"audio\""
		}
	}

	return nil
}
