# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (.mkv, .mp4) und optionale Metadaten (.nfo) über HTTP bereit.
Es gibt kein Web-Interface, keine Datenbank und keine Authentifizierung.

## Start

```bash
go run . -root ./media -addr :8080
