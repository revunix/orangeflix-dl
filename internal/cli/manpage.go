package cli

import (
	"fmt"
	"io"
)

// printManPage writes the man page of the program in the roff format. Save
// it with "orangeflix-dl --man > orangeflix-dl.1" so the shell loads it.
func printManPage(out io.Writer) int {
	fmt.Fprint(out, manRoff)
	return 0
}

const manRoff = `.TH ORANGEFLIX-DL 1 "2026" "orangeflix-dl" "Benutzerbefehle"
.SH NAME
orangeflix-dl \- Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen
.SH SYNOPSIS
.B orangeflix-dl
.RI [ optionen ]
.PP
.B orangeflix-dl
.RI [ optionen ]
.B --format
.I format
.RB [ --episode
.IR n " |"
.B --range
.IR a\-b " |"
.B --year
.IR jahr " |"
.B --since
.IR datum " |"
.B --title
.IR text " |"
.BR --all ] 
.SH BESCHREIBUNG
.B orangeflix-dl
archiviert die Folgen der Formate von Orangeflix für die eigene, private
Nutzung mit gültigem Abo. Ohne Terminal läuft der Headless-Modus für die
Automatisierung, mit Terminal die interaktive Oberfläche.
.SH OPTIONEN
.TP
.BR --list
Alle Formate mit ihrem Status auflisten.
.TP
.BR --format " " \fIformat\fP
Ein Format wählen: Slug (\fImediatheke\fP), Name oder Nummer.
.TP
.BR --episode " " \fIn\fP
Eine einzelne Folge laden.
.TP
.BR --range " " \fIa\-b\fP
Einen Bereich von Folgen laden.
.TP
.BR --year " " \fIjahr\fP
Alle Folgen eines Jahres laden.
.TP
.BR --since " " \fIdatum\fP
Alle Folgen ab dem Tag laden (YYYY-MM-DD).
.TP
.BR --title " " \fItext\fP
Die Folgen laden, deren Titel den Text enthält.
.TP
.BR --all
Alle Folgen des Formats laden.
.TP
.BR --quality " " \fIqualität\fP
.BR best " (Standard), " hd ", " sd ", " mobile " oder " audio "."
.TP
.BR --layout " " \fIlayout\fP
.BR compat " (Standard) oder " flat "."
.TP
.BR --parallel " " \fIn\fP
Bis zu \fIn\fP Downloads gleichzeitig (1 bis 8).
.TP
.BR --delay " " \fIsekunden\fP
Pause zwischen zwei Downloads.
.TP
.BR --thumb
Vorschaubild als JPEG neben das Video legen.
.TP
.BR --meta
Metadaten als JSON neben das Video legen.
.TP
.BR --totp " " \fIcode\fP
Aktueller Code des zweiten Faktors, wenn das Konto ihn verlangt.
.TP
.BR --library
Die gemerkten Formate auflisten.
.TP
.BR --json
Die Ausgabe als JSON (mit \-\-library oder \-\-list).
.TP
.BR --check
Alle gemerkten Formate auf neue Folgen prüfen; Ausgang 1, wenn eine Prüfung
fehlschlägt.
.TP
.BR --download-new
Prüfen und die neuen Folgen laden.
.TP
.BR --add " " \fIformat\fP
Ein Format merken, ohne es zu laden.
.TP
.BR --automatic
Ohne Rückfragen laufen.
.TP
.BR --links-file " " \fIdatei\fP
Formate aus einer Datei lesen (eine Zeile je Format).
.TP
.BR --force-redownload
Vorhandene Dateien verwerfen und erneut laden.
.TP
.BR --completions " " \fIshell\fP
Ein Kompletionsskript ausgeben: \fIbash\fP, \fIzsh\fP oder \fIfish\fP.
.TP
.BR --man
Diese Manpage ausgeben.
.TP
.BR --help
Die Hilfe anzeigen.
.TP
.BR --version
Die Version anzeigen.
.SH UMGEBUNG
.TP
.BR ORANGEFLIX_USER ", " ORANGEFLIX_PASSWORD
Die Zugangsdaten.
.TP
.BR ORANGEFLIX_TOTP
Ein TOTP-Code; wird nur einmal gelesen.
.TP
.BR ORANGEFLIX_QUALITY ", " ORANGEFLIX_LAYOUT ", " ORANGEFLIX_SUBMODULE
Vorgaben für die Dateien und die Feed-Variante.
.TP
.BR ORANGEFLIX_PARALLEL ", " ORANGEFLIX_DELAY ", " ORANGEFLIX_THUMB ", " ORANGEFLIX_META
Vorgaben für das Verhalten der Downloads.
.TP
.BR ORANGEFLIX_DOWNLOADS_DIR ", " ORANGEFLIX_LIBRARY ", " ORANGEFLIX_CREDENTIALS_FILE ", " ORANGEFLIX_CONFIG_FILE
Pfade der Daten.
.TP
.BR ORANGEFLIX_DEBUG_DIR
Verzeichnis für die Diagnose-Datei \fIrequests.log\fP.
.SH KONFIGURATIONSDATEI
.IR ~/.config/orangeflix-dl/config
(Form elektron. "key=value", Vorgaben wie
.BR "quality=hd" ", " "layout=flat" ", " "thumb=1" ", " "parallel=2" )
gilt für jeden Lauf; die Kommandozeile und die Umgebung haben Vorrang.
.SH DATEIEN
Die Untertitel liegen als \fI.de.vtt\fP, das Vorschaubild als \fI.jpg\fP und
die Metadaten als \fI.json\fP neben den Videos.
.SH SIEHE AUCH
Die Hilfe im Programm (\fI.B orangeflix-dl --help\fP) und LEGAL.md im
Quellbaum.
`
