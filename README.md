# PrimeTime

PrimeTime ist ein minimalistischer Media-Server in Go.
Er stellt Video-Dateien (.mkv, .mp4, .m2ts) und optionale Metadaten (.nfo) sowie Untertitel (.srt/.vtt) über HTTP bereit.
Es gibt kein Web-Interface - der Fokus liegt auf einer sauberen REST API für separate Clients.

## ✨ Features

- 🎬 **Video-Streaming** mit Range-Request-Support
- 📝 **NFO-Metadaten** (Kodi-kompatibel)
- 🔐 **Authentication** mit Admin/User-Verwaltung
- 👥 **Multi-User-Support** mit separaten Watch-Histories
- 🎞️ **Transcoding** mit vordefinierten Profilen
- 📺 **TV Shows** mit automatischer Episoden-Gruppierung
- 📁 **Multi-Root** für mehrere Media-Verzeichnisse
- ⭐ **Favorites & Collections** (Playlists)
- 🔍 **Erweiterte Suche** (Genre, Jahr, Rating)
- 🚀 **Minimalistisch** - keine Plugins, kein LiveTV, kein UI
=======

## HTTP-Caching (ETag)

Für Video-Streams und Text-Dateien (z. B. NFO/Untertitel) setzt PrimeTime einen `ETag`, der aus Dateigröße und Änderungszeit berechnet wird.
Wenn der Client einen passenden `If-None-Match` mitsendet, antwortet der Server mit `304 Not Modified`.

## Unterstützte NFO-Typen

PrimeTime liest Kodi-kompatible XML-`*.nfo` Dateien und mappt die wichtigsten Felder:

* `movie`: `title`, `originaltitle`, `plot`, `year`, `rating`, `genre`
* `tvshow`: `title`, `plot`, `genre`
* `episodedetails`: `title`, `plot`, `season`, `episode`, `showtitle`, `rating`
* `musicvideo`: `title`, `album` → `originalTitle`, `artist` → `showTitle`, `plot`, `year`, `rating`, `genre`
* `person`: `name` → `title`, `sortname` → `originalTitle`, `biography` → `plot`, `year`/`born` → `year`

Nicht erkannte Root-Elemente werden als `unknown` gekennzeichnet.

## Episoden-Metadaten aus Dateinamen (Fallback)

Wenn keine `.nfo` vorhanden ist, versucht PrimeTime Episoden-Metadaten aus dem Dateinamen abzuleiten.
Unterstützte Muster (Groß-/Kleinschreibung egal, Trenner wie `.`/`-`/`_`/Leerzeichen erlaubt):

* `S01E02` (z. B. `Meine Serie S01E02`)
* `S01 E02` / `S01.E02`
* `1x02` (z. B. `Meine Serie 1x02`)

Gefundene Werte werden als `title`, `season`, `episode` im JSON von `/items/{id}/nfo` ausgegeben.

## Voraussetzungen

* **Go 1.24** muss installiert sein (entspricht `go.mod`).
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

## Schnellstart

### 1. FFmpeg installieren
Lade FFmpeg herunter und kopiere die Binaries nach `tools/ffmpeg/`:
- Windows: `ffmpeg.exe`, `ffprobe.exe` + alle DLLs
- Linux/macOS: `ffmpeg`, `ffprobe`

### 2. Server starten

**Erster Start (Admin-Passwort wird angezeigt):**
```bash
go run . -root ./media -addr :8080 -db ./data/primetime.db
```

**Ausgabe:**
```
level=info msg="========================================"
level=info msg="FIRST RUN: Admin user created"
level=info msg="Username: admin"
level=info msg="Password: Abc123XyZ789"
level=info msg="IMPORTANT: Save this password securely!"
level=info msg="========================================"
level=info msg="server listening" addr=:8080
```

⚠️ **Speichere das Admin-Passwort!** Es wird nur einmal angezeigt.

### 3. API testen

**Login:**
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Abc123XyZ789"}'
```

**Library abrufen:**
```bash
curl http://localhost:8080/library \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Dokumentation

- 📖 [AUTHENTICATION.md](AUTHENTICATION.md) - Authentifizierungs-System und Benutzerverwaltung

## API Endpoints

### System
```
GET    /health                         - Healthcheck (optional ?json=1 für Details)
GET    /stats                          - Statistiken (optional ?detailed=1)
GET    /version                        - Version
```
**Query-Parameter für `GET /stats`:**
- `detailed`: Wenn `1`, liefert zusätzliche Detailstatistiken.

### Authentication
```
POST   /auth/login                    - Login
POST   /auth/logout                   - Logout
GET    /auth/session                  - Session validieren
GET    /auth/users                    - Benutzer auflisten (Admin)
POST   /auth/users                    - Benutzer erstellen (Admin)
POST   /auth/users/{id}/password      - Passwort ändern
DELETE /auth/users/{id}               - Benutzer löschen (Admin)
```

### Media Library
```
GET    /library                       - Alle Medien
POST   /library                       - Rescan
POST   /library/scan                  - Scan eines Pfads
GET    /library/recent                - Kürzlich hinzugefügt
GET    /library/duplicates            - Duplikate finden
GET    /library/type/{type}           - Filter nach Typ (movie, tvshow, ...)
```

**Query-Parameter für `GET /library`:**
- `q`: Freitextsuche im Titel (Substring).
- `sort`: Sortierung (`title`, `modified`, `size`; Default: `title`).
- `limit`: Maximale Anzahl der Einträge.
- `offset`: Anzahl der Einträge überspringen (Pagination).
- `genre`: Genre-Filter (z. B. `Action`).
- `year`: Jahr-Filter (z. B. `2020`).
- `type`: Typ-Filter (z. B. `movie`, `tvshow`).
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

### Items
```
GET    /items/{id}                    - Media-Details
GET    /items/{id}/exists             - Existiert?
GET    /items/{id}/stream             - Video-Stream
GET    /items/{id}/stream?profile=X   - Transkodierter Stream
GET    /items/{id}/stream.m3u8        - HLS-Playlist
GET    /items/{id}/stream.m3u8?profile=X - HLS-Playlist (Profil)
GET    /items/{id}/nfo                - Metadaten
GET    /items/{id}/nfo/raw            - Raw NFO
GET    /items/{id}/subtitles          - Untertitel
GET    /items/{id}/playback           - Playback-State
POST   /items/{id}/playback           - Playback-State setzen
GET    /items/{id}/watched            - Gesehen?
POST   /items/{id}/watched            - Als gesehen markieren
DELETE /items/{id}/watched            - Gesehen entfernen
GET    /items/{id}/favorite           - Favorit?
POST   /items/{id}/favorite           - Favorit setzen
DELETE /items/{id}/favorite           - Favorit entfernen
GET    /items/{id}/poster             - Poster-Bild
GET    /items/{id}/poster/exists      - Poster vorhanden?
```

### Multi-User
```
GET    /users                         - Alle Benutzer
POST   /users                         - Benutzer erstellen
GET    /users/{id}                    - Benutzer-Details
DELETE /users/{id}                    - Benutzer löschen
```

### TV Shows
```
POST   /shows                         - Auto-Gruppierung
GET    /shows                         - Alle Serien
GET    /shows/{id}                    - Serien-Details
DELETE /shows/{id}                    - Serie löschen
GET    /shows/{id}/seasons            - Staffeln
GET    /shows/{id}/seasons/{season}/episodes - Episoden einer Staffel
GET    /shows/{id}/next-episode       - Nächste Episode
```

### Transcoding
```
GET    /transcoding/profiles          - Alle Profile
POST   /transcoding/profiles          - Profil erstellen
GET    /transcoding/profiles/{id}     - Profil abrufen
DELETE /transcoding/profiles/{id}     - Profil löschen
GET    /transcoding/jobs              - Transcoding-Jobs
```

### Multi-Root
```
GET    /library/roots                 - Alle Roots
POST   /library/roots                 - Root hinzufügen
DELETE /library/roots                 - Root entfernen
POST   /library/roots/{id}/scan       - Root scannen
```

### Playback & Listen
```
GET    /playback                      - Alle Playback-States (optional ?clientId=, ?unfinished=1)
GET    /favorites                     - Favoriten-Liste
GET    /watched                       - Gesehene Items
GET    /collections                   - Collections (Playlists)
POST   /collections                   - Collection erstellen
GET    /collections/{id}              - Collection abrufen
PUT    /collections/{id}              - Collection aktualisieren
DELETE /collections/{id}              - Collection löschen
GET    /collections/{id}/items        - Collection-Items
POST   /collections/{id}/items        - Item hinzufügen
DELETE /collections/{id}/items/{mediaId} - Item entfernen
```

## CLI-Optionen

### Produktivbetrieb

**Kompilierte Binary erstellen:**
```bash
go build -o primetime_server.exe .
```

**Server starten:**
```bash
primetime_server.exe -root ./media -addr :8080 -db ./data/primetime.db
```

### Entwicklung

**Mit Go direkt:**
```bash
go run . -root ./media -addr :8080 -db ./data/primetime.db
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
Pfadregeln für `-db`:

* Der Pfad muss auf eine Datei zeigen (kein Verzeichnis).
* Das Verzeichnis wird bei Bedarf erstellt.
* Die Datenbankdatei wird mit restriktiven Rechten angelegt (z. B. `0600`).
* `:memory:` nutzt eine In‑Memory‑DB ohne Dateipfad.
Weitere Optionen:

* `-scan-interval` (Intervall für automatische Scans; Default: `10m`; `0` deaktiviert die Scans)
* `-no-initial-scan` (überspringt den initialen Scan beim Start)
* `-cors` (aktiviert `Access-Control-Allow-Origin: *`)
* `-json-errors` (JSON-Fehlerantworten statt Plain-Text)
* `-extensions` (kommagetrennte Dateiendungen für den Scan)
* `-db-busy-timeout` (SQLite Busy-Timeout; Default: `5s`; `0` deaktiviert)
* `-db-synchronous` (SQLite Synchronous-Modus; Default: `NORMAL`)
* `-db-cache-size` (SQLite Cache-Size; Default: `-65536` = ca. 64 MiB)
* `-db-read-only` (öffnet die SQLite-DB schreibgeschützt; intern `file:...?...&mode=ro`)
* `-read-only-scan` (erlaubt Scans im Read-only-Modus; Ergebnisse nur im In-Memory-Cache)
* `-sqlite-integrity-check` (führt `PRAGMA integrity_check` aus und beendet sich)
* `-sqlite-vacuum` (führt `VACUUM` aus und beendet sich)
* `-sqlite-vacuum-into` (führt `VACUUM INTO` für ein DB-Backup aus und beendet sich)
* `-sqlite-analyze` (führt `ANALYZE` aus und beendet sich)

### Read-only-Modus

Mit `-db-read-only` wird die Datenbank nur lesend geöffnet. Voraussetzungen und Verhalten:

* Die DB-Datei muss bereits existieren (kein Auto-Create).
* Initialer Scan und periodische Scans sind deaktiviert (außer mit `-read-only-scan`).
* Schreibende Endpoints wie `POST /library` (Rescan) und `POST /items/{id}/playback` liefern `403`.

Mit `-read-only-scan` sind Scans auch im Read-only-Modus möglich. Dabei gilt:

* Scan-Ergebnisse landen ausschließlich im In-Memory-Cache.
* Schreibzugriffe auf die DB (z. B. Scan-Runs, NFOs, Items) bleiben deaktiviert.

### SQLite-Konfiguration (PRAGMA)

Beim Öffnen der Datenbank setzt PrimeTime folgende pragmatische Defaults:

* `PRAGMA journal_mode = WAL;` (bessere Parallelität bei Lesezugriffen)
* `PRAGMA synchronous = NORMAL;` (schneller bei WAL, akzeptabler Schutz für Medienkatalog; über `-db-synchronous` anpassbar)
* `PRAGMA busy_timeout = 5000;` (vermeidet sofortige Lock-Fehler bei parallelen Zugriffen; über `-db-busy-timeout` anpassbar)
* `PRAGMA temp_store = MEMORY;` (temporäre Tabellen/Indizes im RAM für weniger I/O)
* `PRAGMA cache_size = -65536;` (≈ 64 MiB Cache; negative Werte = KiB; über `-db-cache-size` anpassbar)
* `PRAGMA journal_size_limit = 67108864;` (≈ 64 MiB; begrenzt WAL-/Journal-Wachstum)

Die Werte sind auf einen ausgewogenen Mix aus Performance und Sicherheit ausgelegt und können bei Bedarf
an die lokale Hardware oder sehr große Bibliotheken angepasst werden.

**Wichtig:** FFmpeg muss manuell unter `tools/ffmpeg/` installiert werden. Es gibt keinen automatischen Download.

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
### Rate Limits

* `POST /library` ist auf einen manuellen Rescan pro 30 Sekunden begrenzt. Zu frühe Aufrufe erhalten HTTP `429`.
* `POST /items/{id}/playback` (Event `progress`) ist pro `(mediaID, clientId)` auf ein Update alle 5 Sekunden begrenzt. Zu frühe Updates erhalten HTTP `429`.

Playback-`progress`-Updates werden pro `(mediaID, clientId)` im Speicher gedrosselt.
Der Query-Parameter `q` filtert nach Treffern im Titel.
Der Query-Parameter `sort` unterstützt `title`, `modified` und `size` (Default: `title`).
Der Query-Parameter `limit` begrenzt die Anzahl der Einträge; `offset` überspringt die ersten N Einträge.

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

### Windows-Beispiel (sqlite3.exe mit absolutem Pfad)

```bash
C:\Users\Benutzername\Downloads\sqlite-tools-win-x64-3510100\sqlite3.exe "C:\PrimeTime-main\data\primetime.db" ".tables"
```

Alternativ (aus dem Projektverzeichnis):

```bash
cd C:\PrimeTime-main
C:\Users\Benutzername\Downloads\sqlite-tools-win-x64-3510100\sqlite3.exe ".\data\primetime.db" ".tables"
```

### SQLite-Hinweise (sinnvolle Ergänzungen)

* **Pfad klar dokumentieren:** In README/CLI-Beispielen immer den aktiven `-db`‑Pfad mit angeben, damit klar ist, wo die Datenbank liegt.
* **Schema kurz beschreiben:** Kurzer Abschnitt mit den wichtigsten Tabellen (`media_items`, `nfo`) und ihrer Rolle. Das hilft beim Debugging und beim Client‑Abgleich.
* **Backup/Restore:** Ein knapper Hinweis, wie man die DB vor Updates/Rescans sichern kann (z. B. Kopieren der `primetime.db`).
  * Offline-Backup per `VACUUM INTO` (einmalig, ohne Serverstart): `go run . -db ./data/primetime.db -sqlite-vacuum-into ./backup/primetime.db`.
  * Alternativ: `-sqlite-vacuum` optimiert die bestehende DB-Datei.
* **Integritätscheck/Debug:** `go run . -db ./data/primetime.db -sqlite-integrity-check` führt `PRAGMA integrity_check;` aus und beendet sich.
* **Query-Plan-Pflege:** `go run . -db ./data/primetime.db -sqlite-analyze` führt `ANALYZE;` aus (Statistiken für den Query-Planer).
* **Performance bei vielen Medien:** Optional Hinweis auf SQLite‑WAL‑Mode (bei späterem Wachstum), falls parallele Client‑Zugriffe geplant sind.

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

curl "http://localhost:8080/health?json=1"
# Erwartet: {"db":{"connected":true,"readOnly":false},"ffmpeg":{"ready":true},"uptime":123} (uptime in Sekunden)
```

## Troubleshooting (kurz)

* ffmpeg fehlt: sicherstellen, dass `tools/ffmpeg/ffmpeg(.exe)` und `tools/ffmpeg/ffprobe(.exe)` vorhanden sind.
* Build-Probleme: `go mod tidy` ausführen, falls `go.sum`/Module fehlen.
