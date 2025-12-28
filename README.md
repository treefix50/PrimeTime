# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (.mkv, .mp4) und optionale Metadaten (.nfo) über HTTP bereit.
Es gibt kein Web-Interface und keine Authentifizierung.

## Voraussetzungen

* **Go 1.22** muss installiert sein (entspricht `go.mod`).
* **ffmpeg** muss verfügbar sein. Unter Windows wird standardmäßig beim Programmstart ein lokales ffmpeg in `tools/ffmpeg` bereitgestellt (Auto-Download).
  * Der Auto-Download passiert **beim Programmstart** in `internal/ffmpeg/ensure.go`.
  * Er greift **nur unter Windows**. Mit `PRIMETIME_NO_FFMPEG_DOWNLOAD=1` wird er deaktiviert; dann muss ffmpeg lokal vorhanden sein (im `PATH` oder `tools/ffmpeg`).
* `./media` existiert oder wird beim Start erzeugt. Optional wird eine SQLite-DB unter `./data/primetime.db` angelegt.

## Start

```bash
./run.ps1 -root ./media -addr :8080 -db ./data/primetime.db
```

Startet den HTTP-Server und führt einen initialen Scan im `-root`-Verzeichnis aus.
Standardmäßig nutzt PrimeTime eine SQLite-Datenbank unter `./data/primetime.db`.
Der Pfad lässt sich mit `-db` anpassen (z. B. `-db :memory:`).
Weitere Optionen:

* `-scan-interval` (Intervall für automatische Scans; Default: `10m`; `0` deaktiviert die Scans)
* `-cors` (aktiviert `Access-Control-Allow-Origin: *`)

Statt `go run .` sollte das Skript `./run.ps1` genutzt werden.
`run.ps1` führt **keinen** ffmpeg-Download aus; es ruft nur (falls nötig) `go mod tidy` und danach `go run .` auf.
Der **ffmpeg-Auto-Download passiert beim Programmstart** (siehe `internal/ffmpeg/ensure.go`) und
greift **nur unter Windows**. Mit `PRIMETIME_NO_FFMPEG_DOWNLOAD=1` wird der Auto-Download deaktiviert;
dann muss ffmpeg lokal vorhanden sein (im `PATH` oder `tools/ffmpeg`).

## Beispiele/Kommandos

```bash
curl http://localhost:8080/health
# Erwartet: "ok"

curl http://localhost:8080/library
# Erwartet: JSON-Array mit Library-Einträgen

curl "http://localhost:8080/library?q=matrix"  # Filterung über Query möglich
# Erwartet: JSON-Array, gefiltert nach Titel-Substring "matrix"

curl "http://localhost:8080/library?q=alien"
# Erwartet: JSON-Array, gefiltert nach Titel-Substring "alien"

curl -X POST http://localhost:8080/library  # triggert einen Rescan
# Erwartet: Rescan wird angestoßen, Antwort: { "status": "ok" }

curl -I http://localhost:8080/items/{id}/stream
# Erwartet: 200/206 (Range möglich), Stream-Endpoint

curl http://localhost:8080/items/{id}/nfo
# Erwartet: JSON-Metadaten, 404 falls keine NFO existiert

curl http://localhost:8080/items/{id}/nfo/raw
# Erwartet: XML-Text der NFO, 404 falls keine NFO existiert
```
Der Query-Parameter `q` filtert nach Treffern im Titel.

## Smoke-Tests (ohne Medien)

```bash
curl.exe -s -o NUL -w "%{http_code}\n" http://localhost:8080/items/does-not-exist
# Erwartet: 404

curl.exe -s -o NUL -w "%{http_code}\n" http://localhost:8080/items/does-not-exist/stream
# Erwartet: 404

curl.exe -s -o NUL -w "%{http_code}\n" -X POST http://localhost:8080/library  # Rescan
# Erwartet: 200 (Rescan-Trigger)
```

Hinweis: `/health` funktioniert auch ohne Medien. Optionaler Einzeiler:

```bash
curl http://localhost:8080/health
# Erwartet: "ok"
```

## Troubleshooting (kurz)

* ffmpeg fehlt: sicherstellen, dass es im `PATH` liegt oder Auto-Download nicht deaktiviert ist.
* Windows-Auto-Download: `tools/ffmpeg/ffmpeg.exe` und `ffprobe.exe` müssen mehrere MB groß sein; nur wenige KB deuten auf einen defekten Download hin.
* Build-Probleme: `go mod tidy` ausführen, falls `go.sum`/Module fehlen.
