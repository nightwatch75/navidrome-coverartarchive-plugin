# Navidrome Cover Art Archive plugin

A Navidrome metadata plugin that resolves **album cover art** from the
[Cover Art Archive](https://coverartarchive.org) (CAA) using
[MusicBrainz](https://musicbrainz.org) for lookup.

It complements agents like Apple Music and Deezer: when those don't have art for
an album (common for obscure, non-commercial, or self-released records — e.g.
things pulled in from Soulseek), this plugin fills the gap from the
MusicBrainz/CAA ecosystem.

## How it works

`GetAlbumImages` is called by Navidrome with the album name, artist, and (if
known) a MusicBrainz ID. The plugin then:

1. If Navidrome already knows an MBID, it queries CAA directly
   (`/release/{mbid}`, then `/release-group/{mbid}`).
2. Otherwise it searches MusicBrainz for the best matching **release-group**
   (`/ws/2/release-group/?query=releasegroup:"…" AND artist:"…"`).
3. It fetches the CAA listing for that entity and returns the **front cover** in
   the sizes CAA provides: 250 px, 500 px, 1200 px, plus the full-resolution
   original (`size = 0`, which tells Navidrome it's the largest available).

All outbound HTTP goes through Navidrome's `http_send` host service; the plugin
only talks to `musicbrainz.org` and `coverartarchive.org` (see `manifest.json`).

## Layout

| File | Purpose |
| --- | --- |
| `main.go` | Plugin source (implements `metadata.AlbumImagesProvider`). |
| `manifest.json` | Plugin manifest + HTTP host allow-list. |
| `go.mod` / `go.sum` | Module definition. Requires the Navidrome Go PDK. |
| `dist/coverartarchive.ndp` | Prebuilt package (Navidrome v0.62.0 ABI). |
| `tools/harness/` | Standalone Extism runner that executes the built `.wasm` with a real `http_send` implementation — verifies the plugin end-to-end without a running Navidrome. |
| `build.ps1` | Reproducible build: fetches the pinned Navidrome source, compiles, and packages the `.ndp`. |

## Building

The plugin depends on Navidrome's Go PDK (`github.com/navidrome/navidrome/plugins/pdk/go`),
which lives inside the Navidrome source tree. The simplest path is to build it
inside a Navidrome source checkout at the version matching your server (the host
ABI must match).

Requirements: Go 1.24+ (no TinyGo/LLVM needed — standard Go can target
`wasip1` with `-buildmode=c-shared`).

```powershell
# One-shot reproducible build (downloads Navidrome source, builds, packages):
./build.ps1
```

Or manually, inside a Navidrome checkout:

```bash
# Copy main.go, manifest.json, go.mod into navidrome/plugins/examples/coverartarchive-go/
cd navidrome/plugins/examples/coverartarchive-go
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
# Package: a .ndp is just a zip of manifest.json + plugin.wasm at the root
zip -j coverartarchive.ndp manifest.json plugin.wasm
```

## Installing

1. Copy `coverartarchive.ndp` into your Navidrome plugins folder
   (`<DataFolder>/plugins`, e.g. `C:\Navidrome\data\plugins`). Navidrome's file
   watcher discovers it automatically (added **disabled**).
2. Add `coverartarchive` to the `Agents` list in `navidrome.toml`, in priority
   order — e.g. after your primary art source:

   ```toml
   Agents = "artist-nfo-metadata,apple-music,coverartarchive,deezer,spotify,lastfm"
   ```

3. Enable the plugin in **Settings → Plugins** in the Navidrome web UI (or via
   the admin API). It loads live; no restart required.

## Verifying

The harness runs the actual `.wasm` through the Extism runtime (the same one
Navidrome uses), providing a real `http_send` backed by `net/http`:

```powershell
cd tools/harness
go run . ..\..\dist\plugin.wasm   # or a freshly built plugin.wasm
```

Expected output resolves real CAA URLs, e.g. for *OK Computer* by *Radiohead*:

```
size=250   https://coverartarchive.org/release/.../-250.jpg
size=500   https://coverartarchive.org/release/.../-500.jpg
size=1200  https://coverartarchive.org/release/.../-1200.jpg
size=0     https://coverartarchive.org/release/....png
```

## Notes

- MusicBrainz rate-limits to ~1 request/sec and requires a descriptive
  `User-Agent`; both are handled. Navidrome caches agent results, so live usage
  stays well within limits.
- Cover Art Archive image URLs 307-redirect to `archive.org`; Navidrome follows
  these when fetching artwork.
