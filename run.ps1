if (-not (Test-Path .\go.sum)) {
  go mod tidy
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

$ffmpeg = ".\\tools\\ffmpeg\\ffmpeg.exe"
$ffprobe = ".\\tools\\ffmpeg\\ffprobe.exe"

if (-not (Test-Path $ffmpeg)) {
  Write-Error "ffmpeg.exe fehlt: Datei in .\\tools\\ffmpeg\\ ablegen (bin-Ordner aus dem ffmpeg-Archiv kopieren)."
  exit 1
}

if (-not (Test-Path $ffprobe)) {
  Write-Error "ffprobe.exe fehlt: Datei in .\\tools\\ffmpeg\\ ablegen (bin-Ordner aus dem ffmpeg-Archiv kopieren)."
  exit 1
}

& $ffmpeg -version | Out-Null
if ($LASTEXITCODE -ne 0) {
  Write-Error "ffmpeg.exe konnte nicht gestartet werden. Bitte sicherstellen, dass alle DLLs aus dem bin-Ordner mitkopiert wurden."
  exit $LASTEXITCODE
}

& $ffprobe -version | Out-Null
if ($LASTEXITCODE -ne 0) {
  Write-Error "ffprobe.exe konnte nicht gestartet werden. Bitte sicherstellen, dass alle DLLs aus dem bin-Ordner mitkopiert wurden."
  exit $LASTEXITCODE
}

go run .
