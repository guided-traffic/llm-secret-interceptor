---

## Offene Fragen

### Frage 1: Programmiersprache
In welcher Programmiersprache soll der Proxy implementiert werden? (z.B. Go, Rust, Python, Node.js)

**Antwort:** Go

---

### Frage 2: Unterstützte Protokolle/API-Formate
Der Proxy ist generisch und leitet jeden HTTPS-Traffic weiter. Um jedoch Secrets in den Nachrichten zu finden und zu ersetzen, muss er die Payload verstehen können.

Welche API-Formate/Protokolle sollen initial geparst werden können?
- OpenAI Chat Completions API Format (wird auch von GitHub Copilot, Azure OpenAI, und vielen kompatiblen Diensten verwendet)
- Anthropic Messages API Format
- Streaming (Server-Sent Events) Unterstützung?
- Andere?

**Antwort:**
- Das Protokoll-System muss **modular und erweiterbar** sein
- Der Proxy muss das Format **automatisch erkennen** (Auto-Detection)
- Nach Erkennung wird die Payload in ein **standardisiertes internes Format** umgewandelt
- Dieses standardisierte Format wird an den Secret Interceptor Manager weitergeleitet
- Initial: OpenAI-kompatibles Format (deckt Copilot, Azure OpenAI, etc. ab)

---

### Frage 3: Secret-Erkennungsmodule
Welche Secret-Erkennungsmodule sollen initial implementiert werden?
- Regex-basiert (API-Keys, Passwörter in Code, etc.)
- Entropie-basiert
- Bitwarden-Integration
- Andere?

**Antwort:**
- Es gibt einen **Secret Interceptor Manager** der mehrere **Secret Interceptors** verwaltet
- Architektur muss **modular und erweiterbar** sein (Plugin-Architektur)
- Geplante Interceptors:
  - **Entropie-basiert** – erkennt hochentropische Strings
  - **Bitwarden-Integration** – kennt die konkreten Secrets des Nutzers aus seinem Vault
  - Weitere können später hinzugefügt werden
- Jeder Interceptor "flaggt" gefundene Secrets

---

### Frage 4: Konfigurationsformat
In welchem Format soll die Konfiguration erfolgen? (z.B. YAML, JSON, TOML, Environment-Variablen)

**Antwort:** YAML

---

### Frage 5: Persistenz der Mappings
Wie sollen die Secret-zu-Platzhalter-Mappings gespeichert werden? (In-Memory, SQLite, Redis, Datei)

**Antwort:**
- **In-Memory Map** für Single-Instance-Betrieb
- **Redis** für Multi-Instance-Betrieb (horizontale Skalierung)
- Jedes Mapping speichert den **Zeitpunkt der letzten Nutzung**
- **TTL-basierte Löschung**: Nicht mehr benötigte Mappings werden nach einer konfigurierbaren Zeit automatisch gelöscht

---

### Frage 6: Logging und Audit
Soll es ein Logging/Audit-System geben, das protokolliert, welche Secrets erkannt und ersetzt wurden (ohne die Secrets selbst zu loggen)?

**Antwort:**
- **Audit-Log** – protokolliert welcher Interceptor was gefunden hat (ohne die Secrets selbst zu loggen)
- **Konfigurierbar** – Nutzer kann Log-Level und Details nach persönlicher Vorliebe anpassen
- **Prometheus Metriken-Endpunkt** – für Monitoring und Alerting

---

### Frage 7: UI/Dashboard
Soll es eine Web-UI oder ein Dashboard geben, um den Proxy zu konfigurieren und zu überwachen?

**Antwort:** Keine UI – nur YAML-Konfiguration und CLI

---

### Frage 8: Deployment
Wie soll der Proxy deployed werden? (Docker, natives Binary, systemd-Service, etc.)

**Antwort:** Container (Docker)

---

### Frage 9: Streaming-Unterstützung
Wie soll mit Server-Sent Events (SSE) Streaming umgegangen werden, wenn Platzhalter über mehrere Chunks verteilt sein könnten?

**Antwort:**
- **Read-Ahead Buffer mit maximaler Platzhalter-Länge**
- Platzhalter haben ein festes Format mit definierter Maximallänge (z.B. `__SECRET_<hash>__`)
- Buffer behält immer die letzten N Zeichen (= max. Platzhalter-Länge)
- Alles davor ist "sicher" und wird sofort weitergeleitet
- Latenz ist nicht kritisch – ein paar Sekunden Verzögerung sind akzeptabel

---

### Frage 10: Platzhalter-Format
In welchem Format sollen die Platzhalter generiert werden?

**Antwort:**
- **Prefix + Hash** als Standard (z.B. `__SECRET_a1b2c3d4__`)
- Format ist **konfigurierbar** – Nutzer kann Prefix/Suffix anpassen
- Kurz, eindeutig und einfach zu erkennen
