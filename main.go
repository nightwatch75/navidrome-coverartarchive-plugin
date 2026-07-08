// Cover Art Archive plugin for Navidrome.
//
// Resolves album cover art by looking up releases on MusicBrainz and fetching
// images from the Cover Art Archive (CAA). Navidrome calls this agent when it
// needs album art; which agents run and in what order is controlled by the
// Agents list in navidrome.toml (first successful hit wins).
//
// Build with (standard Go 1.24+):
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/metadata"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
)

// MusicBrainz requires a descriptive User-Agent with contact info, otherwise it
// rate-limits or blocks the request.
const userAgent = "navidrome-coverartarchive-plugin/1.0.0 ( https://github.com/sunsetroute1/navidrome-coverartarchive-plugin )"

type caaPlugin struct{}

func init() {
	metadata.Register(&caaPlugin{})
}

var _ metadata.AlbumImagesProvider = (*caaPlugin)(nil)

// ---- MusicBrainz search response ----

type mbReleaseGroupSearch struct {
	ReleaseGroups []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"release-groups"`
}

// ---- Cover Art Archive response ----

type caaResponse struct {
	Images []caaImage `json:"images"`
}

type caaImage struct {
	Front      bool              `json:"front"`
	Image      string            `json:"image"`
	Types      []string          `json:"types"`
	Thumbnails map[string]string `json:"thumbnails"`
}

// httpGetJSON performs a GET via Navidrome's HTTP host service and decodes JSON.
func httpGetJSON(u string, v any) (int, error) {
	resp, err := host.HTTPSend(host.HTTPRequest{
		Method: "GET",
		URL:    u,
		Headers: map[string]string{
			"Accept":     "application/json",
			"User-Agent": userAgent,
		},
		TimeoutMs: 10000,
	})
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != 200 {
		return int(resp.StatusCode), fmt.Errorf("http status %d", resp.StatusCode)
	}
	if v != nil {
		if err := json.Unmarshal(resp.Body, v); err != nil {
			return int(resp.StatusCode), fmt.Errorf("parse json: %w", err)
		}
	}
	return int(resp.StatusCode), nil
}

// lucene escapes characters that are significant inside a quoted MusicBrainz
// (Lucene) query phrase.
func lucene(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

// resolveReleaseGroupMBID searches MusicBrainz for the best release-group match.
func resolveReleaseGroupMBID(name, artist string) (string, error) {
	if name == "" {
		return "", errors.New("album name required")
	}
	q := fmt.Sprintf(`releasegroup:"%s"`, lucene(name))
	if artist != "" {
		q += fmt.Sprintf(` AND artist:"%s"`, lucene(artist))
	}
	u := "https://musicbrainz.org/ws/2/release-group/?fmt=json&limit=5&query=" + url.QueryEscape(q)

	var res mbReleaseGroupSearch
	if _, err := httpGetJSON(u, &res); err != nil {
		return "", err
	}
	if len(res.ReleaseGroups) == 0 {
		return "", errors.New("no release-group match")
	}
	return res.ReleaseGroups[0].ID, nil
}

// caaFetch queries the Cover Art Archive for a release or release-group MBID and
// returns the front cover in the sizes CAA provides.
func caaFetch(kind, mbid string) (*metadata.AlbumImagesResponse, error) {
	u := fmt.Sprintf("https://coverartarchive.org/%s/%s", kind, mbid)

	var res caaResponse
	if _, err := httpGetJSON(u, &res); err != nil {
		return nil, err
	}

	var chosen *caaImage
	for i := range res.Images {
		if res.Images[i].Front {
			chosen = &res.Images[i]
			break
		}
	}
	if chosen == nil && len(res.Images) > 0 {
		chosen = &res.Images[0]
	}
	if chosen == nil {
		return nil, errors.New("no images")
	}

	images := buildImageList(chosen)
	if len(images) == 0 {
		return nil, errors.New("no usable image urls")
	}
	return &metadata.AlbumImagesResponse{Images: images}, nil
}

// buildImageList turns a CAA image entry into Navidrome ImageInfo entries. CAA
// exposes fixed-size thumbnails plus the full-resolution original (Size 0 tells
// Navidrome this is the largest available).
func buildImageList(img *caaImage) []metadata.ImageInfo {
	var out []metadata.ImageInfo
	for _, t := range []struct {
		key  string
		size int32
	}{{"250", 250}, {"500", 500}, {"1200", 1200}} {
		if u, ok := img.Thumbnails[t.key]; ok && u != "" {
			out = append(out, metadata.ImageInfo{URL: u, Size: t.size})
		}
	}
	if img.Image != "" {
		out = append(out, metadata.ImageInfo{URL: img.Image, Size: 0})
	}
	return out
}

// GetAlbumImages resolves cover art for an album. It first tries any MBID
// Navidrome already knows (as a release, then as a release-group), and finally
// falls back to a MusicBrainz search by album + artist name.
func (*caaPlugin) GetAlbumImages(req metadata.AlbumRequest) (*metadata.AlbumImagesResponse, error) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf("CAA GetAlbumImages: name=%q artist=%q mbid=%q", req.Name, req.Artist, req.MBID))

	if req.MBID != "" {
		if resp, err := caaFetch("release", req.MBID); err == nil {
			pdk.Log(pdk.LogInfo, fmt.Sprintf("CAA: art via release MBID %s (%d urls)", req.MBID, len(resp.Images)))
			return resp, nil
		}
		if resp, err := caaFetch("release-group", req.MBID); err == nil {
			pdk.Log(pdk.LogInfo, fmt.Sprintf("CAA: art via release-group MBID %s (%d urls)", req.MBID, len(resp.Images)))
			return resp, nil
		}
		pdk.Log(pdk.LogDebug, fmt.Sprintf("CAA: MBID %s had no art, falling back to search", req.MBID))
	}

	rgID, err := resolveReleaseGroupMBID(req.Name, req.Artist)
	if err != nil {
		pdk.Log(pdk.LogInfo, fmt.Sprintf("CAA: no MusicBrainz match for %q / %q: %v", req.Name, req.Artist, err))
		return nil, errors.New("album not found on MusicBrainz")
	}

	resp, err := caaFetch("release-group", rgID)
	if err != nil {
		pdk.Log(pdk.LogInfo, fmt.Sprintf("CAA: no cover art for release-group %s: %v", rgID, err))
		return nil, errors.New("no cover art in Cover Art Archive")
	}
	pdk.Log(pdk.LogInfo, fmt.Sprintf("CAA: art via searched release-group %s (%d urls)", rgID, len(resp.Images)))
	return resp, nil
}

// main is required by the WASM reactor build; registration happens in init.
func main() {}
