# Konfiguration

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

## CLI-Optionen

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

## Read-only-Modus

Mit `-db-read-only` wird die Datenbank nur lesend geöffnet. Voraussetzungen und Verhalten:

* Die DB-Datei muss bereits existieren (kein Auto-Create).
* Initialer Scan und periodische Scans sind deaktiviert (außer mit `-read-only-scan`).
* Schreibende Endpoints wie `POST /library` (Rescan) und `POST /items/{id}/playback` liefern `403`.

Mit `-read-only-scan` sind Scans auch im Read-only-Modus möglich. Dabei gilt:

* Scan-Ergebnisse landen ausschließlich im In-Memory-Cache.
* Schreibzugriffe auf die DB (z. B. Scan-Runs, NFOs, Items) bleiben deaktiviert.

## SQLite-Konfiguration (PRAGMA)

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

## Audio-Downmixing für Transcoding-Profile

Für Transcoding-Profile lässt sich die Kanalanzahl und optional ein Downmix-Pan-Filter konfigurieren.
PrimeTime setzt dafür bei Bedarf `-ac <n>` und `-af pan=<layout>` in den FFmpeg-Argumenten.

Relevante Felder im Transcoding-Profil (API `POST /transcoding/profiles`):

* `maxAudioChannels`: Maximale Kanalanzahl für den Client. Stereo-Clients sollten hier `2` setzen (ergibt `-ac 2`).
* `audioLayout`: Optionales Pan-Layout für den Downmix. Der Wert wird direkt hinter `pan=` gesetzt.

Beispiel (Stereo-Downmix mit explizitem Pan-Layout):

```json
{
  "name": "stereo-client",
  "videoCodec": "libx264",
  "audioCodec": "aac",
  "maxAudioChannels": 2,
  "audioLayout": "stereo|c0=FL+0.707*FC+0.707*BL|c1=FR+0.707*FC+0.707*BR",
  "container": "mp4"
}
```

## Audio-Normalisierung für Transcoding-Profile

Für Transcoding-Profile kann optional eine Audio-Normalisierung aktiviert werden. Der Wert wird an FFmpeg
als zusätzlicher Filter in `-af` gehängt (z. B. `loudnorm` oder `dynaudnorm`). Ist das Feld leer, wird kein
Normalisierungsfilter gesetzt.

Relevantes Feld im Transcoding-Profil (API `POST /transcoding/profiles`):

* `audioNormalization`: FFmpeg-Filtername, z. B. `loudnorm` oder `dynaudnorm`.

Beispiel (Stereo-Downmix + Loudness-Normalisierung):

```json
{
  "name": "stereo-loudnorm",
  "videoCodec": "libx264",
  "audioCodec": "aac",
  "maxAudioChannels": 2,
  "audioLayout": "stereo",
  "audioNormalization": "loudnorm",
  "container": "mp4"
}
```
