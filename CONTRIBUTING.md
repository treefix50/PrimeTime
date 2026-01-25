# Contributing

Vielen Dank für dein Interesse an PrimeTime! Dieser Abschnitt beschreibt den Build- und Dev-Workflow.

## Produktivbetrieb

**Kompilierte Binary erstellen:**
```bash
go build -o primetime_server.exe .
```

**Server starten:**
```bash
primetime_server.exe -root ./media -addr :8080 -db ./data/primetime.db
```

## Entwicklung

**Mit Go direkt:**
```bash
go run . -root ./media -addr :8080 -db ./data/primetime.db
```
