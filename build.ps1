# Reproducible build for the Cover Art Archive Navidrome plugin.
#
# Downloads the pinned Navidrome source (for the Go PDK), compiles the plugin to
# WASM with standard Go, and packages dist/coverartarchive.ndp.
#
# Requires: Go 1.24+ on PATH. No TinyGo/LLVM needed.
$ErrorActionPreference = 'Stop'

# Navidrome version whose plugin ABI this plugin targets. Match your server.
$ndVersion = 'v0.62.0'

$root   = $PSScriptRoot
$work   = Join-Path $root 'navidrome-src'
$ndDir  = Join-Path $work "navidrome-$($ndVersion.TrimStart('v'))"
$plugDir = Join-Path $ndDir 'plugins\examples\coverartarchive-go'

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is not on PATH. Install Go 1.24+ first: https://go.dev/dl/'
}

if (-not (Test-Path $ndDir)) {
    Write-Host "Downloading Navidrome $ndVersion source..."
    New-Item -ItemType Directory -Force -Path $work | Out-Null
    $zip = Join-Path $work 'nd-src.zip'
    curl.exe -L --fail -s -o $zip "https://codeload.github.com/navidrome/navidrome/zip/refs/tags/$ndVersion"
    tar.exe -xf $zip -C $work
}

Write-Host "Staging plugin into Navidrome tree..."
New-Item -ItemType Directory -Force -Path $plugDir | Out-Null
Copy-Item (Join-Path $root 'main.go')       $plugDir -Force
Copy-Item (Join-Path $root 'manifest.json') $plugDir -Force
Copy-Item (Join-Path $root 'go.mod')        $plugDir -Force
Copy-Item (Join-Path $root 'go.sum')        $plugDir -Force

Write-Host "Building plugin.wasm..."
Push-Location $plugDir
try {
    $env:GOOS = 'wasip1'; $env:GOARCH = 'wasm'
    go build -buildmode=c-shared -o plugin.wasm .
} finally {
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host "Packaging dist/coverartarchive.ndp..."
$dist = Join-Path $root 'dist'
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$stage = Join-Path $work 'stage'
Remove-Item $stage -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $stage | Out-Null
Copy-Item (Join-Path $plugDir 'manifest.json') $stage
Copy-Item (Join-Path $plugDir 'plugin.wasm')   $stage
$ndp = Join-Path $dist 'coverartarchive.ndp'
$zip2 = Join-Path $work 'coverartarchive.zip'
Remove-Item $ndp, $zip2 -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $stage 'manifest.json'), (Join-Path $stage 'plugin.wasm') -DestinationPath $zip2 -Force
Move-Item $zip2 $ndp -Force

Write-Host "Done: $ndp"
