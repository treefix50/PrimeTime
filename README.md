# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (.mkv, .mp4, …) und Metadaten (.nfo) über HTTP bereit.
Es gibt kein Web-Dashboard – Clients greifen direkt über HTTP zu.

## Starten

```bash
go run . -root ./media -addr :8080
### Commands
GET /health
GET /library
GET /items/{id}
GET /items/{id}/stream
GET /items/{id}/nfo
GET /items/{id}/nfo/raw
