# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (.mkv, .mp4) und optionale Metadaten (.nfo) über HTTP bereit.
Es gibt kein Web-Interface und keine Authentifizierung.

## Start

```bash
go run . -root ./media -addr :8080 -db ./data/primetime.db
```

Standardmäßig nutzt PrimeTime eine SQLite-Datenbank unter `./data/primetime.db`.
Der Pfad lässt sich mit `-db` anpassen (z. B. `-db :memory:`).

Kurze Befehle:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/library
curl -I http://localhost:8080/items/{id}/stream
```
