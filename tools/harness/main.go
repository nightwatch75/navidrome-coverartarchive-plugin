// Standalone verification harness for the Cover Art Archive Navidrome plugin.
//
// It loads plugin.wasm with the Extism Go SDK (the same runtime Navidrome uses)
// and provides a real `http_send` host function backed by net/http, mirroring
// Navidrome's plugins/host/http_gen.go. It then calls nd_get_album_images with a
// few test albums and prints the resulting cover-art URLs.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	extism "github.com/extism/go-sdk"
)

type HTTPRequest struct {
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	Headers           map[string]string `json:"headers"`
	NoFollowRedirects bool              `json:"noFollowRedirects"`
	Body              []byte            `json:"body"`
	TimeoutMs         int32             `json:"timeoutMs"`
}

type HTTPResponse struct {
	StatusCode int32             `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

type httpSendRequest struct {
	Request HTTPRequest `json:"request"`
}

type httpSendResponse struct {
	Result *HTTPResponse `json:"result,omitempty"`
	Error  string        `json:"error,omitempty"`
}

func doHTTP(req HTTPRequest) (*HTTPResponse, error) {
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	hreq, err := http.NewRequestWithContext(ctx, strings.ToUpper(req.Method), req.URL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		hreq.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}
	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return &HTTPResponse{StatusCode: int32(resp.StatusCode), Headers: headers, Body: b}, nil
}

func main() {
	ctx := context.Background()

	httpSend := extism.NewHostFunctionWithStack(
		"http_send",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			reqBytes, err := p.ReadBytes(stack[0])
			write := func(v any) {
				out, _ := json.Marshal(v)
				ptr, werr := p.WriteBytes(out)
				if werr != nil {
					stack[0] = 0
					return
				}
				stack[0] = ptr
			}
			if err != nil {
				write(httpSendResponse{Error: err.Error()})
				return
			}
			var sr httpSendRequest
			if err := json.Unmarshal(reqBytes, &sr); err != nil {
				write(httpSendResponse{Error: err.Error()})
				return
			}
			fmt.Printf("  [http_send] %s %s\n", sr.Request.Method, sr.Request.URL)
			res, err := doHTTP(sr.Request)
			if err != nil {
				write(httpSendResponse{Error: err.Error()})
				return
			}
			fmt.Printf("  [http_send] -> %d (%d bytes)\n", res.StatusCode, len(res.Body))
			write(httpSendResponse{Result: res})
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypePTR},
	)

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{extism.WasmFile{Path: os.Args[1]}},
	}
	cfg := extism.PluginConfig{EnableWasi: true}
	plugin, err := extism.NewPlugin(ctx, manifest, cfg, []extism.HostFunction{httpSend})
	if err != nil {
		fmt.Println("NewPlugin error:", err)
		os.Exit(1)
	}
	defer plugin.Close(ctx)

	type album struct{ Name, Artist, MBID string }
	tests := []album{
		{Name: "OK Computer", Artist: "Radiohead"},
		{Name: "Pablo Honey", Artist: "Radiohead"},
		{Name: "Discovery", Artist: "Daft Punk"},
	}

	for _, a := range tests {
		time.Sleep(1500 * time.Millisecond) // respect MusicBrainz ~1 req/sec rate limit
		fmt.Printf("\n=== GetAlbumImages: %q by %q ===\n", a.Name, a.Artist)
		in, _ := json.Marshal(map[string]string{"name": a.Name, "artist": a.Artist, "mbid": a.MBID})
		exit, out, err := plugin.Call("nd_get_album_images", in)
		if err != nil {
			fmt.Printf("  RESULT: exit=%d err=%v\n", exit, err)
			continue
		}
		if exit != 0 {
			fmt.Printf("  RESULT: non-zero exit=%d out=%s\n", exit, string(out))
			continue
		}
		var resp struct {
			Images []struct {
				URL  string `json:"url"`
				Size int32  `json:"size"`
			} `json:"images"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			fmt.Printf("  RESULT: parse error: %v raw=%s\n", err, string(out))
			continue
		}
		fmt.Printf("  RESULT: %d image(s):\n", len(resp.Images))
		for _, img := range resp.Images {
			fmt.Printf("    size=%-5d %s\n", img.Size, img.URL)
		}
	}
}
