package main

import (
	"fmt"
)

type vsize struct {
	w, h int
}

func (v *vsize) String() string {
	return fmt.Sprintf("%dx%d", v.w, v.h)
}

func (v *vsize) Scale() string {
	return fmt.Sprintf("scale=w=%d:h=%d", v.w, v.h)
}

// isOver returns true if width or height is over the provided value
func (v *vsize) isOver(n int) bool {
	return v.w > n || v.h > n
}

func (v *vsize) smaller() *vsize {
	if v.w < v.h {
		// reverse it, make it smaller, reverse it again
		return v.reverse().smaller().reverse()
	}

	steps := []int{4320, 2160, 1440, 1080, 720, 480, 360, 240} //, 144}

	for _, nh := range steps {
		if v.h <= nh {
			continue
		}
		// resize to 1920x1080
		if v.w*nh%v.h != 0 {
			continue
		}
		nw := v.w * nh / v.h
		if nw&1 != 0 {
			continue
		}
		if nw <= 145 || nh <= 145 {
			// too small
			return nil
		}
		return &vsize{w: nw, h: nh}
	}
	return nil
}

func (v *vsize) reverse() *vsize {
	if v == nil {
		return nil
	}
	return &vsize{w: v.h, h: v.w}
}

func (v *vsize) bitrate(framerate, bitsPerPixel float64) uint64 {
	// compute ideal bitrate for h264: 0.1 bit per pixel
	// 1080p@60fps is ~6Mbps
	return uint64(float64(v.w) * float64(v.h) * framerate * bitsPerPixel)
}

func (v *vsize) variants() []*hlsVariant {
	// make a smart variant depending on the size
	if v.isOver(1280) {
		return []*hlsVariant{
			&hlsVariant{size: v, codec: AV1},
			&hlsVariant{size: v, codec: HEVC},
		}
	} else {
		return []*hlsVariant{
			&hlsVariant{size: v, codec: H264},
		}
	}
}
