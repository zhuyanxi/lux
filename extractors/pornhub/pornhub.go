package pornhub

import (
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/dop251/goja"
	"github.com/pkg/errors"

	"github.com/iawia002/lux/extractors"
	"github.com/iawia002/lux/request"
	"github.com/iawia002/lux/utils"
)

func init() {
	extractors.Register("pornhub", New())
}

type pornhubData struct {
	DefaultQuality bool   `json:"defaultQuality"`
	Format         string `json:"format"`
	VideoURL       string `json:"videoUrl"`
	Quality        string `json:"quality"`
}

type mediaDefinition struct {
	Format   string `json:"format"`
	VideoURL string `json:"videoUrl"`
}

type extractor struct{}

// New returns a pornhub extractor.
func New() extractors.Extractor {
	return &extractor{}
}

// Extract is the main function to extract the data.
func (e *extractor) Extract(url string, option extractors.Options) ([]*extractors.Data, error) {
	res, err := request.Request(http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer res.Body.Close() // nolint

	var reader io.ReadCloser
	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		reader, _ = gzip.NewReader(res.Body)
	case "deflate":
		reader = flate.NewReader(res.Body)
	default:
		reader = res.Body
	}
	defer reader.Close() // nolint

	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	html := string(b)

	cookiesArr := make([]string, 0)
	cookies := res.Cookies()

	for _, c := range cookies {
		cookiesArr = append(cookiesArr, c.Name+"="+c.Value)
	}

	var title string
	desc := utils.MatchOneOf(html, `<span class="inlineFree">(.+?)</span>`)
	if len(desc) > 1 {
		title = desc[1]
	} else {
		title = "pornhub video"
	}

	reg, err := regexp.Compile(`<script\b[^>]*>([\s\S]*?)</script>`)
	if err != nil {
		return nil, errors.WithStack(extractors.ErrInvalidRegularExpression)
	}

	matchers := reg.FindAllStringSubmatch(html, -1)
	var encryptedScript string

	for _, scripts := range matchers {
		script := scripts[1]
		if !strings.Contains(script, "flashvars_") {
			continue
		} else {
			encryptedScript = script
			break
		}
	}

	mediaDefinitions, err := evaluateMediaDefinitions(encryptedScript)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var mp4MediaDefinition *mediaDefinition

	for _, mediaDefinition := range mediaDefinitions {
		if mediaDefinition.Format == "mp4" {
			mp4MediaDefinition = &mediaDefinition
		}
	}

	if mp4MediaDefinition == nil {
		return nil, errors.New("can not found media")
	}

	resApi, err := request.Get(mp4MediaDefinition.VideoURL, url, map[string]string{
		"Cookie": strings.Join(cookiesArr, "; "),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	pornhubs := make([]pornhubData, 0)

	if err := json.Unmarshal([]byte(resApi), &pornhubs); err != nil {
		return nil, errors.WithStack(err)
	}

	streams := make(map[string]*extractors.Stream, len(pornhubs))

	for _, data := range pornhubs {
		size, err := request.Size(data.VideoURL, url)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		urlData := &extractors.Part{
			URL:  data.VideoURL,
			Size: size,
			Ext:  data.Format,
		}

		streams[data.Quality] = &extractors.Stream{
			Parts:   []*extractors.Part{urlData},
			Size:    size,
			Quality: data.Quality,
		}
	}

	return []*extractors.Data{
		{
			Site:    "Pornhub pornhub.com",
			Title:   title,
			Type:    extractors.DataTypeVideo,
			Streams: streams,
			URL:     url,
		},
	}, nil
}

func evaluateMediaDefinitions(script string) ([]mediaDefinition, error) {
	flashID := regexp.MustCompile(`flashvars_\d+`).FindString(script)
	if flashID == "" {
		return nil, extractors.ErrURLParseFailed
	}

	vm := goja.New()
	_, err := vm.RunString(`var playerObjList = {};` + script + fmt.Sprintf(`;var __VM__OUTPUT = JSON.stringify(%s.mediaDefinitions)`, flashID))
	if err != nil {
		return nil, err
	}

	mediaDefinitions := make([]mediaDefinition, 0)
	if err := json.Unmarshal([]byte(vm.Get("__VM__OUTPUT").String()), &mediaDefinitions); err != nil {
		return nil, err
	}

	return mediaDefinitions, nil
}
