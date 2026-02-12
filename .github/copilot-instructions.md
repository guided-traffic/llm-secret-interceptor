# LLM Secret Interceptor - Projektdokumentation

## Projektübersicht

Ein Proxy-Server, der speziell für die Nutzung mit LLMs (z.B. VSCode Copilot) zugeschnitten ist. Der Proxy:

- Hängt sich als Man-in-the-Middle zwischen Client und LLM-Cloud-Provider
- Durchsucht User-Nachrichten nach exponierten Secrets
- Ersetzt gefundene Secrets durch eindeutige Platzhalter vor dem Weiterleiten
- Wandelt Platzhalter in LLM-Antworten wieder in die Original-Secrets zurück
- Besitzt eine eigene Self-Signed CA für TLS-Interception
- Bietet erweiterbare und konfigurierbare Module zur Secret-Erkennung (z.B. Bitwarden, Entropie-basiert)


---

## Architekturentscheidungen

_Wird nach Klärung der Fragen ergänzt_

---

## Technische Details

_Wird nach Klärung der Fragen ergänzt_

---

## Entwicklungsrichtlinien für Copilot

### Sprache

**WICHTIG:** Alle Texte im Repository müssen auf **Englisch** geschrieben werden. Das gilt für:
- Code-Kommentare
- Commit-Messages
- Dokumentation
- Variable-/Funktionsnamen
- Log-Messages
- Error-Messages

Der Benutzer kann auf Deutsch kommunizieren, aber alle eingecheckten Inhalte müssen Englisch sein.

### Testing und CI-Konsistenz

**WICHTIG:** Verwende IMMER die Makefile-Targets für Tests und Checks, um Konsistenz mit der CI zu gewährleisten.

| Aufgabe | Makefile-Target | NICHT verwenden |
|---------|-----------------|-----------------|
| Unit Tests | `make test` | `go test ./...` |
| Linting | `make lint` | `golangci-lint run` |
| Security Scan | `make gosec` | `gosec ./...` |
| Vulnerability Check | `make vuln` | `govulncheck ./...` |
| Cyclomatic Complexity | `make cyclo` | `gocyclo ./...` |
| Alle Checks | `make all` | - |

Die Tool-Versionen sind im Makefile gepinnt und werden von Renovate verwaltet. So ist sichergestellt, dass lokal und in der CI dieselben Versionen verwendet werden.
