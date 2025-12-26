# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (z. B. `.mkv`, `.mp4`) und Metadaten (`.nfo`) über HTTP bereit.
Es gibt kein Web-Dashboard – Clients greifen direkt über HTTP zu.

PrimeTime dient ausschließlich als Anlaufstelle für Media-Clients
(z. B. VLC, Kodi, Infuse oder eigene Clients).

---

## Starten

```bash
go run . -root ./media -addr :8080

#Media-Verzeichnis

PrimeTime scannt beim Start das angegebene Media-Verzeichnis.

## media/
  Movie.Name.2024.mkv
  Movie.Name.2024.nfo
  Show.Name.S01E01.mkv
  Show.Name.S01E01.nfo


Video- und .nfo-Dateien müssen den gleichen Basisnamen haben

Unterverzeichnisse werden rekursiv gescannt

#HTTP API
GET /health

Health-Check (prüft, ob der Server läuft).

Antwort:

ok

GET /library

Gibt alle gefundenen Media-Items als JSON zurück.

GET /items/{id}

Details zu einem einzelnen Media-Item.

GET /items/{id}/stream

Videostream des Media-Items.

Unterstützt HTTP Range Requests

Kann direkt in Media-Playern geöffnet werden (VLC, Kodi, Infuse)

Beispiel:

http://localhost:8080/items/<id>/stream

GET /items/{id}/nfo

Gibt die geparste .nfo Datei als JSON zurück (falls vorhanden).

GET /items/{id}/nfo/raw

Gibt die rohe .nfo Datei zurück (XML oder Text).

Hinweise

PrimeTime ist bewusst minimal gehalten

Keine Authentifizierung

Keine Datenbank

Kein Web-Interface

Konzipiert für lokale oder private Netzwerke
