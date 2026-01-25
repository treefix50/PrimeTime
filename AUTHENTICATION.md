# Authentifizierung

PrimeTime nutzt Token-basierte Sessions. Geschützte Endpunkte erwarten den Header:

```
Authorization: Bearer <token>
```

## Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"DEIN_PASSWORT"}'
```

Die Antwort enthält das Session-Token. Dieses Token muss für weitere API-Aufrufe mitgegeben werden.

## Session prüfen

```bash
curl http://localhost:8080/auth/session \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Logout

```bash
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Benutzerverwaltung (Admin)

```bash
curl http://localhost:8080/auth/users \
  -H "Authorization: Bearer YOUR_TOKEN"
```
