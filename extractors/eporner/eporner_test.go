package eporner

import (
	"testing"

	"github.com/iawia002/lux/extractors"
	"github.com/iawia002/lux/test"
)

func TestDownload(t *testing.T) {
	tests := []struct {
		name string
		args test.Args
	}{
		{
			name: "normal test",
			args: test.Args{
				URL:     "https://www.eporner.com/video-mbubfvXYFip/dirtywivesclub-becky-bandini/",
				Quality: "1080p",
				Size:    1525510307,
				Title:   "DirtyWivesClub - Becky Bandini - EPORNER",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := New().Extract(tt.args.URL, extractors.Options{})
			test.CheckError(t, err)
			test.Check(t, tt.args, data[0])
		})
	}
}

func TestGetSrcParsesNestedDownloadLinks(t *testing.T) {
	html := `<div class="dloaddivcol">
		<span class="download-av1"><a href="/dload/id/240/240p-av1.mp4">Download MP4 (240p, AV1, 53.53 MB)</a></span>
		<span class="download-h264"> or <a href="/dload/id/240/240p.mp4"> MP4 (240p, h264, 86.16 MB)</a></span>
	</div>`

	srcs := getSrc(html)
	if len(srcs) != 2 {
		t.Fatalf("getSrc() returned %d sources, want 2", len(srcs))
	}

	toBytes := func(megabytes float64) int64 {
		return int64(megabytes * 1024 * 1024)
	}
	want := []struct {
		url  string
		size int64
	}{
		{url: "/dload/id/240/240p-av1.mp4", size: toBytes(53.53)},
		{url: "/dload/id/240/240p.mp4", size: toBytes(86.16)},
	}
	for i, src := range srcs {
		if src.url != want[i].url {
			t.Errorf("source %d URL = %q, want %q", i, src.url, want[i].url)
		}
		if src.quality != "240p" {
			t.Errorf("source %d quality = %q, want %q", i, src.quality, "240p")
		}
		if src.size != want[i].size {
			t.Errorf("source %d size = %d, want %d", i, src.size, want[i].size)
		}
	}
}
