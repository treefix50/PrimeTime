# Support

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
