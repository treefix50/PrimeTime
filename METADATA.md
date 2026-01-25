# Metadaten

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
