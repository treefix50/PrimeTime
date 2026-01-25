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

## 🚀 Schnellstart

### 1) FFmpeg installieren
Lade FFmpeg herunter und kopiere die Binaries nach `tools/ffmpeg/`:
- Windows: `ffmpeg.exe`, `ffprobe.exe` + alle DLLs
- Linux/macOS: `ffmpeg`, `ffprobe`

### 2) Server starten

**Erster Start (Admin-Passwort wird angezeigt):**
```bash
go run . -root ./media -addr :8080 -db ./data/primetime.db
```

### 3) API testen

**Login:**
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"DEIN_PASSWORT"}'
```

**Library abrufen:**
```bash
curl http://localhost:8080/library \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 📚 Dokumentation

- 📖 [AUTHENTICATION.md](AUTHENTICATION.md) - Authentifizierung & Sessions
- 📖 [API.md](API.md) - Alle Endpoints, Query-Parameter, Beispiele
- 📖 [CONFIGURATION.md](CONFIGURATION.md) - Voraussetzungen, FFmpeg, CLI-Optionen, Read-only-Modus
- 📖 [METADATA.md](METADATA.md) - NFO-Mapping, Episoden-Fallback, HTTP-Caching
- 📖 [SUPPORT.md](SUPPORT.md) - Checks, Smoke-Tests, Troubleshooting
- 📖 [CONTRIBUTING.md](CONTRIBUTING.md) - Build- & Dev-Workflow
