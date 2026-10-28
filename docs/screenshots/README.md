# Screenshots

Aufnahmen der Oberfläche, gerendert aus der Terminalausgabe in die Schrift
FiraCode Nerd Font Regular, im dunklen Erscheinungsbild. Aufgenommen bei
112 Spalten × 38 Zeilen.

| Datei | Bildschirm |
|-------|------------|
| `00-intro` | das Intro mit dem Verschlusslogo beim Start |
| `01-anmeldung` | das Anmeldefenster: E-Mail, Passwort, 2FA-Code, Erscheinungsbild |
| `02-formate` | die Formatenliste mit Angaben, Protokoll und Konto |
| `03-folgen` | die Episodenliste des Formats, nach Jahren gruppiert |
| `04-aktion` | das Menü beim Klick auf eine Folge |
| `05-qualitaet` | das Qualitätenfenster mit Auflösung und Dateigröße |
| `06-speicherort` | das Speicherortfenster mit dem Pfad und der Ordnerliste |
| `07-hilfe` | das Hilfefenster mit allen Tasten |
| `08-downloads` | die Downloadliste mit Fortschrittsbalken und Restschätzung |
| `09-fertig` | der Fertig-Bildschirm des Laufs |
| `10-suche` | die Volltextsuche über die Folgen aller Formate |

## Erneuern

Die Bilder stammen aus der Terminalausgabe des Programms, nicht aus einem
Terminalprogramm. Ein kurzes Hilfsprogramm setzt die Ansicht mit
Testdaten, das Werkzeug in `/tmp/ansi2png` (nicht Teil des Projekts)
zeichnet die ANSI-Ausgabe als PNG:

1. Falls `/tmp/ansi2png` nicht mehr existiert, wieder aufbauen: ein kleines
   Go-Programm, das eine ANSI-Datei (Steuerzeichen und Färbung) mit einer
   Schrift als PNG zeichnet.
2. Eine Vorrichtung als Testdatei in `internal/tui` ablegen, die das Modell
   mit Testdaten füllt und `View()` aller Bildschirme in `.ans`-Dateien
   schreibt.
3. Ausgeben mit:

   ```bash
   ansi2png bild.ans docs/screenshots/<name>.png FiraCodeNerdFont-Regular.ttf 112
   ```
