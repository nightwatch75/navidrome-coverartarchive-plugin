# navidrome-coverartarchive-plugin

**Album cover art for Navidrome — from MusicBrainz and the Cover Art Archive.**

A free, open-source Navidrome metadata plugin (`.ndp`) that fills album-art gaps
left by Apple Music, Deezer, and other commercial catalogs. Ideal for obscure,
non-commercial, or self-released music (including downloads from Soulseek).

| | |
| --- | --- |
| **What it does** | Resolves album cover art via MusicBrainz lookup → Cover Art Archive |
| **What it does not do** | Artist photos, lyrics, biographies, or scrobbling |
| **Cost** | Free — MusicBrainz + CAA are free; no API keys |
| **Navidrome** | v0.62.0+ (plugin system / WASM metadata agents) |
| **Download** | [`dist/coverartarchive.ndp`](dist/coverartarchive.ndp) — drop in, enable, done |

## Why this exists

Navidrome's built-in agents and popular plugins (Apple Music, Deezer) cover
mainstream releases well. They often miss:

- Obscure or niche releases
- Self-released / Bandcamp-only albums
- Mis-tagged or inconsistently named files from P2P sources
- Anything well catalogued on MusicBrainz but absent from iTunes/Deezer

The [Cover Art Archive](https://coverartarchive.org) is the canonical open
artwork repository for MusicBrainz. This plugin queries it so Symfonium,
Finamp, and the Navidrome web UI show real cover art for those albums.

## Quick install

1. Download [`dist/coverartarchive.ndp`](dist/coverartarchive.ndp).
2. Copy it to your Navidrome plugins folder (`<DataFolder>/plugins`).
3. Add `coverartarchive` to your agent chain in `navidrome.toml`:

   ```toml
   Agents = "apple-music,coverartarchive,deezer"
   ```

   Place it **after** your primary commercial source so CAA acts as a fallback.

4. Open Navidrome → **Settings → Plugins** → enable **coverartarchive**.

No restart required. Navidrome's file watcher discovers new `.ndp` files
automatically (they start disabled until you toggle them on).

## How it works

When Navidrome needs album art, it calls the plugin's `GetAlbumImages` with the
album name, artist, and optional MusicBrainz ID:

1. **Known MBID** → query CAA directly (`/release/{mbid}`, then `/release-group/{mbid}`).
2. **No MBID** → search MusicBrainz for the best matching release-group
   (`releasegroup:"Album" AND artist:"Artist"`).
3. Return the **front cover** from CAA in 250 / 500 / 1200 px plus full resolution.

Outbound HTTP is restricted to `musicbrainz.org` and `coverartarchive.org`
(see `manifest.json`). Navidrome caches agent results.

## Verify it's working

**In Navidrome:** pick an album that Apple Music/Deezer miss but MusicBrainz
knows. After enabling the plugin, art should appear in the web UI and Subsonic
clients (Symfonium, etc.). You may need to refresh or re-open the album once.

**Standalone (no Navidrome needed):**

```powershell
cd tools/harness
go run . ..\..\dist\coverartarchive.ndp   # extracts and runs plugin.wasm
```

Or point at a freshly built `plugin.wasm`. Expected output includes real CAA URLs:

```
=== GetAlbumImages: "OK Computer" by "Radiohead" ===
  RESULT: 4 image(s):
    size=250   https://coverartarchive.org/release/.../-250.jpg
    size=500   https://coverartarchive.org/release/.../-500.jpg
    size=1200  https://coverartarchive.org/release/.../-1200.jpg
    size=0     https://coverartarchive.org/release/....png
```

## Build from source

Requires Go 1.24+ (standard Go — no TinyGo). The plugin must be built against
the Navidrome PDK version matching your server (ABI compatibility).

```powershell
./build.ps1
```

Output: `dist/coverartarchive.ndp`.

Manual build details and project layout are in the sections below.

## Project layout

| Path | Purpose |
| --- | --- |
| `main.go` | Plugin source (`metadata.AlbumImagesProvider`) |
| `manifest.json` | Navidrome plugin manifest + HTTP allow-list |
| `dist/coverartarchive.ndp` | Prebuilt binary for Navidrome v0.62.0 |
| `build.ps1` | Reproducible build script |
| `tools/harness/` | Extism test runner (same WASM runtime as Navidrome) |

## Agent priority

Order in `Agents` matters — Navidrome tries sources left-to-right and stops at
the first hit. Recommended:

```toml
Agents = "artist-nfo-metadata,apple-music,coverartarchive,deezer,spotify,lastfm"
```

- **apple-music** / **deezer** — fast, good for mainstream catalog
- **coverartarchive** — fallback for MusicBrainz-catalogued releases they miss

## Notes

- MusicBrainz rate-limits to ~1 req/sec; the plugin sends a proper
  `User-Agent`. Navidrome's cache keeps live usage well within limits.
- CAA image URLs redirect to `archive.org`; Navidrome follows these when
  fetching artwork.
- Plugin ID (and agent name) is **`coverartarchive`** — derived from the
  `.ndp` filename, not the manifest display name.

## License

MIT — see [LICENSE](LICENSE).

## Links

- [Navidrome](https://www.navidrome.org/)
- [Navidrome plugin docs](https://www.navidrome.org/docs/developers/plugins/)
- [Cover Art Archive](https://coverartarchive.org/)
- [MusicBrainz](https://musicbrainz.org/)
