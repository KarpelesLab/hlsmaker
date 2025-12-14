package main

import (
	"testing"
)

func TestSmallerSizes(t *testing.T) {
	tests := []struct {
		name     string
		input    vsize
		expected []vsize // expected chain of smaller sizes
	}{
		{
			name:  "3840x1610 (non-standard aspect ratio)",
			input: vsize{w: 3840, h: 1610},
			expected: []vsize{
				{w: 3434, h: 1440},
				{w: 2574, h: 1080},
				{w: 1716, h: 720},
				{w: 1144, h: 480},
				{w: 858, h: 360},
				{w: 572, h: 240},
				{w: 380, h: 160},
			},
		},
		{
			name:  "1920x1080 (standard 16:9)",
			input: vsize{w: 1920, h: 1080},
			expected: []vsize{
				{w: 1280, h: 720},
				{w: 852, h: 480},
				{w: 638, h: 360},
				{w: 424, h: 240},
				{w: 282, h: 160},
			},
		},
		{
			name:  "3840x2160 (4K 16:9)",
			input: vsize{w: 3840, h: 2160},
			expected: []vsize{
				{w: 2560, h: 1440},
				{w: 1920, h: 1080},
				{w: 1280, h: 720},
				{w: 852, h: 480},
				{w: 638, h: 360},
				{w: 424, h: 240},
				{w: 282, h: 160},
			},
		},
		{
			name:  "1080x1920 (vertical video)",
			input: vsize{w: 1080, h: 1920},
			expected: []vsize{
				{w: 720, h: 1280},
				{w: 480, h: 852},
				{w: 360, h: 638},
				{w: 240, h: 424},
				{w: 160, h: 282},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := &tc.input
			for i, expected := range tc.expected {
				smaller := v.smaller()
				if smaller == nil {
					t.Errorf("step %d: expected %v but got nil", i, expected)
					return
				}
				if smaller.w != expected.w || smaller.h != expected.h {
					t.Errorf("step %d: expected %v but got %v", i, expected, smaller)
				}
				v = smaller
			}
			// Verify no more sizes after expected chain
			if final := v.smaller(); final != nil {
				t.Errorf("expected no more sizes after chain, but got %v", final)
			}
		})
	}
}

func TestSmallerGeneratesSizes(t *testing.T) {
	// Test that problematic sizes now generate smaller variants
	problematicSizes := []vsize{
		{w: 3840, h: 1610},
		{w: 2560, h: 1070},
		{w: 1920, h: 803},
	}

	for _, input := range problematicSizes {
		v := &input
		smaller := v.smaller()
		if smaller == nil {
			t.Errorf("%v should generate a smaller size but got nil", input)
			continue
		}
		t.Logf("%v -> %v", input, smaller)

		// Verify aspect ratio is approximately preserved (within 1%)
		origRatio := float64(input.w) / float64(input.h)
		newRatio := float64(smaller.w) / float64(smaller.h)
		diff := (newRatio - origRatio) / origRatio
		if diff < -0.01 || diff > 0.01 {
			t.Errorf("%v -> %v: aspect ratio changed by %.2f%% (too much)", input, smaller, diff*100)
		}

		// Verify dimensions are even
		if smaller.w%2 != 0 {
			t.Errorf("%v -> %v: width is not even", input, smaller)
		}
		if smaller.h%2 != 0 {
			t.Errorf("%v -> %v: height is not even", input, smaller)
		}
	}
}

func TestSmallerChain(t *testing.T) {
	// Test that we can generate a full chain of sizes from 3840x1610
	v := &vsize{w: 3840, h: 1610}
	count := 0
	t.Logf("Starting: %v", v)
	for v != nil {
		smaller := v.smaller()
		if smaller == nil {
			break
		}
		count++
		t.Logf("  -> %v", smaller)
		v = smaller
	}
	if count == 0 {
		t.Error("3840x1610 should generate at least one smaller size")
	}
	t.Logf("Generated %d smaller sizes", count)
}
