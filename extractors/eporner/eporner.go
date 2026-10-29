package eporner

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"

	"github.com/iawia002/lux/extractors"
	"github.com/iawia002/lux/parser"
	"github.com/iawia002/lux/request"
	"github.com/iawia002/lux/utils"
)

func init() {
	extractors.Register("eporner", New())
}

const (
	downloadclass = ".dloaddivcol"
)

type src struct {
	url     string
	quality string
	sizestr string
	size    int64
}

func getSrcMeta(text string) *src {
	s := &src{}
	sti := strings.Index(text, "(")
	ste := strings.Index(text, ")")
	if sti < 0 || ste <= sti {
		return s
	}

	itext := text[sti+1 : ste]
	strs := strings.Split(itext, ",")
	if len(strs) < 2 {
		return s
	}
	s.quality = strings.TrimSpace(strs[0])
	s.sizestr = strings.TrimSpace(strs[len(strs)-1])

	if s.sizestr == "" {
		return s
	}

	valunit := strings.Fields(s.sizestr)
	if len(valunit) == 0 {
		return s
	}
	val, err := strconv.ParseFloat(valunit[0], 64)
	if err != nil {
		return s
	}

	unit := ""
	if len(valunit) > 1 {
		unit = valunit[1]
	}
	switch unit {
	case "KB":
		s.size = int64(val * 1024)
	case "MB":
		s.size = int64(val * 1024 * 1024)
	case "GB":
		s.size = int64(val * 1024 * 1024 * 1024)
	default:
		s.size = int64(val)
	}
	return s
}

func getSrc(html string) []*src {
	srcs := []*src{}
	d, err := parser.GetDoc(html)
	if err != nil {
		return nil
	}

	d.Find(downloadclass).Find("a").Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return
		}

		sr := getSrcMeta(a.Text())
		if sr.quality == "" {
			return
		}
		sr.url = href
		srcs = append(srcs, sr)
	})

	return srcs
}

type extractor struct{}

// New returns a eporner extractor.
func New() extractors.Extractor {
	return &extractor{}
}

// Extract is the main function to extract the data.
func (e *extractor) Extract(u string, option extractors.Options) ([]*extractors.Data, error) {
	html, err := request.Get(u, u, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var title string
	desc := utils.MatchOneOf(html, `<title>(.+?)</title>`)
	if len(desc) > 1 {
		title = desc[1]
	} else {
		title = "eporner"
	}
	uu, err := url.Parse(u)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	srcs := getSrc(html)
	if len(srcs) == 0 {
		return nil, errors.WithStack(extractors.ErrURLParseFailed)
	}
	streams := make(map[string]*extractors.Stream, len(srcs))
	for _, src := range srcs {
		srcurl := uu.Scheme + "://" + uu.Host + src.url
		// skipping an extra HEAD request to the URL.
		// size, err := request.Size(srcurl, u)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		urlData := &extractors.Part{
			URL:  srcurl,
			Size: src.size,
			Ext:  "mp4",
		}
		streams[src.quality] = &extractors.Stream{
			Parts:   []*extractors.Part{urlData},
			Size:    src.size,
			Quality: src.quality,
		}
	}
	return []*extractors.Data{
		{
			Site:    "EPORNER eporner.com",
			Title:   title,
			Type:    extractors.DataTypeVideo,
			Streams: streams,
			URL:     u,
		},
	}, nil
}
