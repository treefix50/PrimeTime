# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (.mkv, .mp4) und optionale Metadaten (.nfo) über HTTP bereit.
Es gibt kein Web-Interface und keine Authentifizierung.

## Voraussetzungen

* **Go 1.22** muss installiert sein (entspricht `go.mod`).
* **ffmpeg** muss lokal vorhanden sein und manuell unter `./tools/ffmpeg` abgelegt werden.
  * Der Ordner `tools/ffmpeg` ist im ZIP bereits vorhanden (leer), damit die Dateien direkt dort abgelegt werden können.
  * **Windows (FFmpeg-Builds ZIP, Ordner enthält `bin/`, `lib/`, `include/`)**:
    1. ZIP herunterladen und entpacken.
    2. **Alle Dateien aus `bin/`** nach `tools/ffmpeg/` kopieren.
       * Muss enthalten: `ffmpeg.exe`, `ffprobe.exe` **und alle `.dll`‑Dateien** aus `bin/`.
  * **Linux/macOS**:
    1. Archiv/Installationspaket entpacken.
    2. **Binaries aus `bin/`** nach `tools/ffmpeg/` kopieren (`ffmpeg` und `ffprobe`).
  * Es gibt **keinen** Auto-Download mehr; ohne diese Dateien startet PrimeTime nicht.
* `./media` existiert oder wird beim Start erzeugt. Optional wird eine SQLite-DB unter `./data/primetime.db` angelegt.

## Start

### Windows (PowerShell)

```bash
./run.ps1 -root ./media -addr :8080 -db ./data/primetime.db
```

Erwartete Struktur nach dem manuellen Kopieren (Windows-Beispiel):

```
tools/
  ffmpeg/
    ffmpeg.exe
    ffprobe.exe
    avcodec-62.dll
    avdevice-62.dll
    avfilter-11.dll
    avformat-62.dll
    avutil-60.dll
    swresample-6.dll
    swscale-9.dll
    ... (alle weiteren DLLs aus dem bin-Ordner)
```

Startet den HTTP-Server und führt einen initialen Scan im `-root`-Verzeichnis aus.
Standardmäßig nutzt PrimeTime eine SQLite-Datenbank unter `./data/primetime.db`.
Der Pfad lässt sich mit `-db` anpassen (z. B. `-db :memory:`).
Weitere Optionen:

* `-scan-interval` (Intervall für automatische Scans; Default: `10m`; `0` deaktiviert die Scans)
* `-cors` (aktiviert `Access-Control-Allow-Origin: *`)

Statt `go run .` sollte unter Windows das Skript `./run.ps1` genutzt werden.
`run.ps1` prüft zuerst, ob `tools/ffmpeg/ffmpeg.exe` und `tools/ffmpeg/ffprobe.exe` vorhanden und ausführbar sind
(inklusive der benötigten `.dll`‑Dateien im selben Ordner).
Anschließend wird `go run .` gestartet. ffmpeg wird **nicht** automatisch heruntergeladen.

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

curl.exe -X POST http://localhost:8080/library  # triggert einen Rescan (PowerShell: echtes curl)
# Erwartet: Rescan wird angestoßen, Antwort: { "status": "ok" }

curl -I http://localhost:8080/items/{id}/stream
# Erwartet: 200/206 (Range möglich), Stream-Endpoint

curl http://localhost:8080/items/{id}/nfo
# Erwartet: JSON-Metadaten, 404 falls keine NFO existiert

curl http://localhost:8080/items/{id}/nfo/raw
# Erwartet: XML-Text der NFO, 404 falls keine NFO existiert
```
Der Query-Parameter `q` filtert nach Treffern im Titel.

## Checks (ffmpeg & SQLite)

```bash
tools/ffmpeg/ffmpeg -version
# Erwartet: Versionsausgabe von ffmpeg

tools/ffmpeg/ffprobe -version
# Erwartet: Versionsausgabe von ffprobe

sqlite3 ./data/primetime.db ".tables"
# Erwartet: Liste der Tabellen (z. B. items, scans, meta)

sqlite3 ./data/primetime.db "SELECT COUNT(*) FROM items;"
# Erwartet: Anzahl der gefundenen Media-Items
```

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

* ffmpeg fehlt: sicherstellen, dass `tools/ffmpeg/ffmpeg(.exe)` und `tools/ffmpeg/ffprobe(.exe)` vorhanden sind.
* Build-Probleme: `go mod tidy` ausführen, falls `go.sum`/Module fehlen.
