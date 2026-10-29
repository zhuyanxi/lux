package pornhub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iawia002/lux/extractors"
	"github.com/iawia002/lux/test"
)

func TestPornhub(t *testing.T) {
	tests := []struct {
		name string
		args test.Args
	}{
		{
			name: "normal test",
			args: test.Args{
				URL:   "https://www.pornhub.com/view_video.php?viewkey=ph5cb5fc41c6ebd",
				Title: "Must watch Milf drilled by the fireplace",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			New().Extract(tt.args.URL, extractors.Options{})
		})
	}
}

func TestEvaluateMediaDefinitionsSupportsOptionalChaining(t *testing.T) {
	script := `var player = playerObjList?.missing; var flashvars_123 = {mediaDefinitions: [{format: "mp4", videoUrl: "https://example.com/video.mp4"}]};`

	mediaDefinitions, err := evaluateMediaDefinitions(script)
	if err != nil {
		t.Fatalf("evaluateMediaDefinitions() error = %v", err)
	}
	if len(mediaDefinitions) != 1 {
		t.Fatalf("evaluateMediaDefinitions() returned %d definitions, want 1", len(mediaDefinitions))
	}
	if mediaDefinitions[0].Format != "mp4" || mediaDefinitions[0].VideoURL != "https://example.com/video.mp4" {
		t.Errorf("evaluateMediaDefinitions() returned %+v, want the fixture media definition", mediaDefinitions[0])
	}
}

func TestExtractUsesPageAsMediaReferer(t *testing.T) {
	apiReferer := make(chan string, 1)
	videoReferer := make(chan string, 1)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			_, _ = fmt.Fprintf(w, `<script>var flashvars_123 = {mediaDefinitions: [{format: "mp4", videoUrl: %q}]};</script>`, server.URL+"/api")
		case "/api":
			apiReferer <- r.Header.Get("Referer")
			body, err := json.Marshal([]pornhubData{{
				Format:   "mp4",
				VideoURL: server.URL + "/video",
				Quality:  "240P",
			}})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(body)
		case "/video":
			videoReferer <- r.Header.Get("Referer")
			w.Header().Set("Content-Length", "123")
			_, _ = w.Write(make([]byte, 123))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	pageURL := server.URL + "/page"
	if _, err := New().Extract(pageURL, extractors.Options{}); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if got := <-apiReferer; got != pageURL {
		t.Errorf("API Referer = %q, want %q", got, pageURL)
	}
	if got := <-videoReferer; got != pageURL {
		t.Errorf("video Referer = %q, want %q", got, pageURL)
	}
}
