# Publish navidrome-coverartarchive-plugin to GitHub (run once after `gh auth login`).
#
# Creates a public repo, sets description/topics, and pushes main.
$ErrorActionPreference = 'Stop'
$env:Path = "C:\gotools\mingit\cmd;C:\Program Files\GitHub CLI;$env:Path"

$repoName = 'navidrome-coverartarchive-plugin'
$desc = 'Navidrome plugin: album cover art from MusicBrainz + Cover Art Archive (fallback for obscure/self-released music)'
$topics = 'navidrome,navidrome-plugin,musicbrainz,cover-art-archive,album-art,wasm,go,self-hosted,music-server'

Set-Location $PSScriptRoot

gh auth status 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host 'Run: gh auth login' -ForegroundColor Yellow
    gh auth login -h github.com -p https -w
}

if (-not (git remote get-url origin 2>$null)) {
    gh repo create $repoName `
        --public `
        --source=. `
        --remote=origin `
        --description $desc `
        --push
} else {
    git push -u origin main
}

gh repo edit --description $desc --add-topic ($topics -split ',') --enable-issues=true 2>$null

$url = gh repo view --json url -q .url
Write-Host "Published: $url" -ForegroundColor Green
Write-Host ""
Write-Host "Optional — tag v1.0.0 + attach the .ndp for one-click download:"
Write-Host "  gh release create v1.0.0 dist/coverartarchive.ndp --title v1.0.0 --notes 'Initial release'"
