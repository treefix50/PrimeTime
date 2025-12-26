# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (.mkv, .mp4) und optionale Metadaten (.nfo) über HTTP bereit.
Es gibt kein Web-Interface und keine Authentifizierung.

## Start

```bash
./run.ps1 -root ./media -addr :8080 -db ./data/primetime.db
```

Standardmäßig nutzt PrimeTime eine SQLite-Datenbank unter `./data/primetime.db`.
Der Pfad lässt sich mit `-db` anpassen (z. B. `-db :memory:`).

Statt `go run .` sollte das Skript `./run.ps1` genutzt werden.

Kurze Befehle:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/library
curl -I http://localhost:8080/items/{id}/stream
```

Zusätzliche Smoke-Tests (ohne Medien):

```bash
curl.exe -s -o NUL -w "%{http_code}\n" http://localhost:8080/items/does-not-exist
curl.exe -s -o NUL -w "%{http_code}\n" http://localhost:8080/items/does-not-exist/stream
curl.exe -s -o NUL -w "%{http_code}\n" -X POST http://localhost:8080/library
```
