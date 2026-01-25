# API

**Hinweis:** Geschützte Endpunkte benötigen den Header `Authorization: Bearer <token>`. In den Listen steht `(Session)` für eine gültige Session, `(Admin)` für Admin-Rechte.

## System
```
GET    /health                         - Healthcheck (optional ?json=1 für Details)
GET    /stats                          - Statistiken (optional ?detailed=1) (Session)
GET    /version                        - Version
```
**Query-Parameter für `GET /stats`:**
- `detailed`: Wenn `1`, liefert zusätzliche Detailstatistiken.

## Authentication
```
POST   /auth/login                    - Login
POST   /auth/logout                   - Logout (Session)
GET    /auth/session                  - Session validieren (Session)
GET    /auth/users                    - Benutzer auflisten (Session, Admin)
POST   /auth/users                    - Benutzer erstellen (Session, Admin)
POST   /auth/users/{id}/password      - Passwort ändern (Session)
DELETE /auth/users/{id}               - Benutzer löschen (Session, Admin)
```

## Media Library
```
GET    /library                       - Alle Medien (Session)
POST   /library                       - Rescan (Session)
POST   /library/scan                  - Scan eines Pfads (Session)
GET    /library/recent                - Kürzlich hinzugefügt (Session)
GET    /library/duplicates            - Duplikate finden (Session)
GET    /library/type/{type}           - Filter nach Typ (movie, tvshow, ...) (Session)
```

**Query-Parameter für `GET /library`:**
- `q`: Freitextsuche im Titel (Substring).
- `sort`: Sortierung (`title`, `modified`, `size`; Default: `title`).
- `limit`: Maximale Anzahl der Einträge.
- `offset`: Anzahl der Einträge überspringen (Pagination).
- `genre`: Genre-Filter (z. B. `Action`).
- `year`: Jahr-Filter (z. B. `2020`).
- `type`: Typ-Filter (z. B. `movie`, `tvshow`).
- `rating`: Mindestbewertung (0–10).

**Body für `POST /library/scan`:**
```json
{ "path": "Serien/Star Trek" }
```
`path` kann relativ zum Root oder absolut angegeben werden.

**Beispiele (Pagination/Filter):**
```bash
curl "http://localhost:8080/library?limit=25&offset=50"
curl "http://localhost:8080/library?genre=Action&year=2020"
curl "http://localhost:8080/library?type=movie&rating=7.5"
```

## Items
```
GET    /items/{id}                    - Media-Details (Session)
GET    /items/{id}/exists             - Existiert? (Session)
GET    /items/{id}/stream             - Video-Stream (Session)
GET    /items/{id}/stream?profile=X   - Transkodierter Stream (Session)
GET    /items/{id}/stream.m3u8        - HLS-Playlist (Session)
GET    /items/{id}/stream.m3u8?profile=X - HLS-Playlist (Profil) (Session)
GET    /items/{id}/nfo                - Metadaten (Session)
GET    /items/{id}/nfo/raw            - Raw NFO (Session)
GET    /items/{id}/subtitles          - Untertitel (Session)
GET    /items/{id}/playback           - Playback-State (Session)
POST   /items/{id}/playback           - Playback-State setzen (Session)
GET    /items/{id}/watched            - Gesehen? (Session)
POST   /items/{id}/watched            - Als gesehen markieren (Session)
DELETE /items/{id}/watched            - Gesehen entfernen (Session)
GET    /items/{id}/favorite           - Favorit? (Session)
POST   /items/{id}/favorite           - Favorit setzen (Session)
DELETE /items/{id}/favorite           - Favorit entfernen (Session)
GET    /items/{id}/poster             - Poster-Bild (Session)
GET    /items/{id}/poster/exists      - Poster vorhanden? (Session)
```

## Multi-User
```
GET    /users                         - Alle Benutzer (Session, Admin)
POST   /users                         - Benutzer erstellen (Session, Admin)
GET    /users/{id}                    - Benutzer-Details (Session, Admin)
DELETE /users/{id}                    - Benutzer löschen (Session, Admin)
```

## TV Shows
```
POST   /shows                         - Auto-Gruppierung (Session)
GET    /shows                         - Alle Serien (Session)
GET    /shows/{id}                    - Serien-Details (Session)
DELETE /shows/{id}                    - Serie löschen (Session)
GET    /shows/{id}/seasons            - Staffeln (Session)
GET    /shows/{id}/seasons/{season}/episodes - Episoden einer Staffel (Session)
GET    /shows/{id}/next-episode       - Nächste Episode (Session)
```

## Transcoding
```
GET    /transcoding/profiles          - Alle Profile (Session)
POST   /transcoding/profiles          - Profil erstellen (Session)
GET    /transcoding/profiles/{id}     - Profil abrufen (Session)
DELETE /transcoding/profiles/{id}     - Profil löschen (Session)
GET    /transcoding/jobs              - Transcoding-Jobs (Session)
```

## Multi-Root
```
GET    /library/roots                 - Alle Roots (Session)
POST   /library/roots                 - Root hinzufügen (Session)
DELETE /library/roots                 - Root entfernen (Session)
POST   /library/roots/{id}/scan       - Root scannen (Session)
```

## Playback & Listen
```
GET    /playback                      - Alle Playback-States (optional ?clientId=, ?unfinished=1) (Session)
GET    /favorites                     - Favoriten-Liste (Session)
GET    /watched                       - Gesehene Items (Session)
GET    /collections                   - Collections (Playlists) (Session)
POST   /collections                   - Collection erstellen (Session)
GET    /collections/{id}              - Collection abrufen (Session)
PUT    /collections/{id}              - Collection aktualisieren (Session)
DELETE /collections/{id}              - Collection löschen (Session)
GET    /collections/{id}/items        - Collection-Items (Session)
POST   /collections/{id}/items        - Item hinzufügen (Session)
DELETE /collections/{id}/items/{mediaId} - Item entfernen (Session)
```

## Beispiele/Kommandos

```bash
curl http://localhost:8080/health
# Erwartet: "ok"

curl "http://localhost:8080/health?json=1"
# Erwartet: {"db":{"connected":true,"readOnly":false},"ffmpeg":{"ready":true},"uptime":123} (uptime in Sekunden)

curl http://localhost:8080/version
# Erwartet: Versionsinfos (JSON)

curl http://localhost:8080/stats
# Erwartet: Statistikdaten (JSON)

curl http://localhost:8080/library
# Erwartet: JSON-Array mit Library-Einträgen

curl "http://localhost:8080/library?q=matrix"  # Filterung über Query möglich
# Erwartet: JSON-Array, gefiltert nach Titel-Substring "matrix"

curl "http://localhost:8080/library?q=alien"
# Erwartet: JSON-Array, gefiltert nach Titel-Substring "alien"

curl "http://localhost:8080/library?sort=title"
# Erwartet: JSON-Array, sortiert nach Titel (Standard)

curl "http://localhost:8080/library?sort=modified"
# Erwartet: JSON-Array, sortiert nach Änderungsdatum (neueste zuerst)

curl "http://localhost:8080/library?sort=size"
# Erwartet: JSON-Array, sortiert nach Größe (größte zuerst)

curl "http://localhost:8080/library?limit=25"
# Erwartet: JSON-Array, maximal 25 Einträge

curl "http://localhost:8080/library?limit=25&offset=50"
# Erwartet: JSON-Array, Einträge 51-75 (pagination)

curl.exe -X POST http://localhost:8080/library  # triggert einen Rescan (PowerShell: echtes curl)
# Erwartet: Rescan wird angestoßen, Antwort: { "status": "ok" } (Rate-Limit: HTTP 429)

curl -X POST http://localhost:8080/library/scan \
  -H "Content-Type: application/json" \
  -d '{ "path": "Serien/Star Trek" }'
# Erwartet: Partial-Scan des Teilbaums (Pfad relativ zu -root oder absolut), Antwort: { "status": "ok" }

curl -I http://localhost:8080/items/{id}/stream
# Erwartet: 200/206 (Range möglich), Stream-Endpoint

curl http://localhost:8080/items/{id}/exists
# Erwartet: { "exists": true/false }

curl http://localhost:8080/items/{id}/nfo
# Erwartet: JSON-Metadaten, 404 falls keine NFO existiert

curl http://localhost:8080/items/{id}/nfo/raw
# Erwartet: XML-Text der NFO, 404 falls keine NFO existiert

curl http://localhost:8080/items/{id}/subtitles
# Erwartet: Text der Untertitel (.vtt bevorzugt, sonst .srt), 404 falls keine Untertitel existieren

curl "http://localhost:8080/items/{id}/playback?clientId=my-player"
# Erwartet: Playback-Status inkl. lastPlayedAt (Unix) und optional percentComplete

curl -X POST "http://localhost:8080/items/{id}/playback?clientId=my-player" \
  -H "Content-Type: application/json" \
  -d '{ "event": "progress", "positionSeconds": 123, "durationSeconds": 456, "lastPlayedAt": 1718611200, "percentComplete": 27.0 }'
# Erwartet: Playback-Update (POST, clientId ist Pflicht, percentComplete optional, Rate-Limit: HTTP 429)
```

## Rate Limits

* `POST /library` ist auf einen manuellen Rescan pro 30 Sekunden begrenzt. Zu frühe Aufrufe erhalten HTTP `429`.
* `POST /items/{id}/playback` (Event `progress`) ist pro `(mediaID, clientId)` auf ein Update alle 5 Sekunden begrenzt. Zu frühe Updates erhalten HTTP `429`.

Playback-`progress`-Updates werden pro `(mediaID, clientId)` im Speicher gedrosselt.
Der Query-Parameter `q` filtert nach Treffern im Titel.
Der Query-Parameter `sort` unterstützt `title`, `modified` und `size` (Default: `title`).
Der Query-Parameter `limit` begrenzt die Anzahl der Einträge; `offset` überspringt die ersten N Einträge.

## Erweiterte Features

### Collections & Favorites
```bash
# Collection erstellen
curl -X POST http://localhost:8080/collections \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Meine Favoriten", "description": "Beste Filme"}'

# Zu Favoriten hinzufügen
curl -X POST http://localhost:8080/favorites/{mediaId} \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Erweiterte Suche
```bash
# Nach Genre suchen
curl "http://localhost:8080/library?genre=Action" \
  -H "Authorization: Bearer YOUR_TOKEN"

# Nach Jahr filtern
curl "http://localhost:8080/library?year=2020" \
  -H "Authorization: Bearer YOUR_TOKEN"

# Nach Rating filtern
curl "http://localhost:8080/library?rating=8.0" \
  -H "Authorization: Bearer YOUR_TOKEN"
```
