# LLM Secret Interceptor - Projektplan

## 📋 Übersicht

Dieser Projektplan beschreibt die Implementierungsphasen für den LLM Secret Interceptor Proxy.

**Geschätzter Gesamtaufwand:** 6-8 Wochen (1 Entwickler)

---

## 🎯 Meilensteine

| # | Meilenstein | Beschreibung | Ziel-KW |
|---|-------------|--------------|---------|
| M1 | Grundstruktur | Projektstruktur, Build-System, CI/CD | KW 1 |
| M2 | Proxy Core | Basis-Proxy mit TLS-Interception | KW 2 |
| M3 | Protokoll-Layer | Auto-Detection & standardisiertes Format | KW 3 |
| M4 | Secret Interception | Manager + Entropy Interceptor | KW 4 |
| M5 | Mapping & Streaming | Storage + Read-Ahead Buffer | KW 5 |
| M6 | Observability | Logging, Audit, Prometheus | KW 6 |
| M7 | Bitwarden Integration | Zweiter Interceptor | KW 7 |
| M8 | Production Ready | Docker, Docs, Testing | KW 8 |

---

## 📦 Phase 1: Projektstruktur (M1)

### Aufgaben

- [ ] **1.1** Go-Modul initialisieren (`go mod init`)
- [ ] **1.2** Projektstruktur anlegen:
  ```
  llm-secret-interceptor/
  ├── cmd/
  │   └── proxy/
  │       └── main.go
  ├── internal/
  │   ├── config/
  │   ├── proxy/
  │   ├── protocol/
  │   ├── interceptor/
  │   ├── storage/
  │   └── metrics/
  ├── pkg/
  │   └── placeholder/
  ├── configs/
  │   └── config.example.yaml
  ├── certs/
  │   └── .gitkeep
  ├── Dockerfile
  ├── docker-compose.yaml
  ├── Makefile
  └── README.md
  ```
- [ ] **1.3** Makefile mit Build-Targets erstellen
- [ ] **1.4** GitHub Actions CI/CD Pipeline
- [ ] **1.5** Basis-Dependencies hinzufügen:
  - `gopkg.in/yaml.v3` (Config)
  - `github.com/rs/zerolog` (Logging)
  - `github.com/prometheus/client_golang` (Metrics)
- [ ] **1.6** Config-Loader implementieren (YAML parsing)

### Deliverables
- Kompilierbares Go-Projekt
- Funktionierende CI-Pipeline
- Config-Loading

---

## 🔐 Phase 2: Proxy Core (M2)

### Aufgaben

- [ ] **2.1** Self-Signed CA Generator implementieren
  - Automatische CA-Erstellung beim ersten Start
  - CA-Zertifikat und Private Key speichern
- [ ] **2.2** Dynamische Zertifikatsgenerierung pro Host
  - On-the-fly Zertifikate für abgefangene Domains
  - Zertifikat-Cache (In-Memory)
- [ ] **2.3** HTTPS Proxy Server implementieren
  - CONNECT-Methode für HTTPS-Tunneling
  - TLS-Interception (MITM)
- [ ] **2.4** Request/Response Handler Grundstruktur
  - Request abfangen → verarbeiten → weiterleiten
  - Response abfangen → verarbeiten → zurückgeben
- [ ] **2.5** Passthrough für nicht-relevante Requests
  - Nicht-LLM-Traffic unverändert durchleiten
- [ ] **2.6** Unit Tests für Proxy-Komponenten

### Deliverables
- Funktionierender HTTPS-Proxy mit TLS-Interception
- CA-Zertifikat für Client-Installation

---

## 🔌 Phase 3: Protokoll-Layer (M3)

### Aufgaben

- [ ] **3.1** Protocol Interface definieren:
  ```go
  type ProtocolHandler interface {
      CanHandle(req *http.Request) bool
      ParseRequest(body []byte) (*StandardMessage, error)
      ParseResponse(body []byte) (*StandardMessage, error)
      SerializeRequest(msg *StandardMessage) ([]byte, error)
      SerializeResponse(msg *StandardMessage) ([]byte, error)
  }
  ```
- [ ] **3.2** Standardisiertes internes Message-Format definieren
- [ ] **3.3** Protocol Registry implementieren (Auto-Detection)
- [ ] **3.4** OpenAI Chat Completions Handler implementieren
  - Request-Parsing (messages array)
  - Response-Parsing (choices array)
- [ ] **3.5** Anthropic Messages Handler implementieren (optional, später)
- [ ] **3.6** Unit Tests für Protocol-Handler

### Deliverables
- Modulares Protokoll-System
- OpenAI-Format Unterstützung
- Auto-Detection funktioniert

---

## 🔍 Phase 4: Secret Interception (M4)

### Aufgaben

- [ ] **4.1** SecretInterceptor Interface definieren:
  ```go
  type SecretInterceptor interface {
      Name() string
      Detect(text string) []DetectedSecret
      Configure(config map[string]interface{}) error
  }
  ```
- [ ] **4.2** Secret Interceptor Manager implementieren
  - Registrierung von Interceptors
  - Sequenzielle Ausführung aller aktiven Interceptors
  - Aggregation der Ergebnisse
- [ ] **4.3** Entropy-basierter Interceptor implementieren
  - Shannon-Entropie Berechnung
  - Konfigurierbarer Schwellenwert
  - Minimale/Maximale String-Länge
- [ ] **4.4** Placeholder Generator implementieren
  - Hash-Generierung (z.B. erste 8 Zeichen SHA256)
  - Konfigurierbares Prefix/Suffix
- [ ] **4.5** Secret Replacer implementieren
  - Text durchsuchen und Secrets ersetzen
  - Position-Tracking für mehrere Secrets
- [ ] **4.6** Unit Tests für Interceptors und Replacer

### Deliverables
- Plugin-fähiges Interceptor-System
- Funktionierender Entropy-Interceptor
- Secrets werden durch Platzhalter ersetzt

---

## 💾 Phase 5: Mapping & Streaming (M5)

### Aufgaben

- [ ] **5.1** Storage Interface definieren:
  ```go
  type MappingStore interface {
      Store(placeholder, secret string) error
      Lookup(placeholder string) (string, bool)
      Touch(placeholder string) error  // TTL update
      Cleanup() error  // Expired entries entfernen
  }
  ```
- [ ] **5.2** In-Memory Store implementieren
  - sync.Map oder RWMutex-geschützte Map
  - TTL-Tracking mit Timestamp
  - Background-Goroutine für Cleanup
- [ ] **5.3** Redis Store implementieren
  - go-redis/redis Integration
  - TTL via Redis EXPIRE
  - Connection Pooling
- [ ] **5.4** Read-Ahead Buffer für Streaming implementieren
  - Feste Buffer-Größe basierend auf max. Platzhalter-Länge
  - Chunk-Aggregation
  - Sichere Abschnitte sofort weiterleiten
- [ ] **5.5** SSE (Server-Sent Events) Handling
  - Event-Stream parsing
  - Chunk-weise Verarbeitung
- [ ] **5.6** Response-Platzhalter zurück in Secrets wandeln
- [ ] **5.7** Integration Tests für End-to-End Flow

### Deliverables
- Persistente Mappings (Memory + Redis)
- Streaming funktioniert korrekt
- Platzhalter werden in Responses zurückgewandelt

---

## 📊 Phase 6: Observability (M6)

### Aufgaben

- [ ] **6.1** Strukturiertes Logging mit zerolog
  - Log-Level konfigurierbar
  - JSON-Format für Production
- [ ] **6.2** Audit-Log implementieren
  - Welcher Interceptor hat was gefunden
  - Zeitstempel, Request-ID
  - NIEMALS Secrets loggen!
- [ ] **6.3** Prometheus Metriken implementieren
  - `llm_proxy_requests_total`
  - `llm_proxy_secrets_detected_total` (mit Interceptor-Label)
  - `llm_proxy_secrets_replaced_total`
  - `llm_proxy_mapping_store_size`
  - `llm_proxy_request_duration_seconds` (Histogram)
- [ ] **6.4** Health-Check Endpoint (`/health`)
- [ ] **6.5** Metrics Endpoint (`/metrics`)
- [ ] **6.6** Grafana Dashboard JSON erstellen (optional)

### Deliverables
- Umfassendes Logging-System
- Prometheus-Metriken verfügbar
- Health-Checks für Container-Orchestrierung

---

## 🔑 Phase 7: Bitwarden Integration (M7)

### Aufgaben

- [ ] **7.1** Bitwarden CLI/API Integration recherchieren
- [ ] **7.2** Bitwarden Interceptor implementieren
  - Vault-Zugriff (read-only)
  - Secret-Liste cachen
  - Exakte Matches finden
- [ ] **7.3** Sichere Credential-Handhabung
  - Master-Password via Environment
  - Session-Token Management
- [ ] **7.4** Vault-Refresh Mechanismus
  - Periodisches Neu-Laden der Secrets
  - Konfigurierbare Intervalle
- [ ] **7.5** Unit Tests mit Mock-Vault

### Deliverables
- Funktionierender Bitwarden-Interceptor
- Sichere Credential-Verwaltung

---

## 🚀 Phase 8: Production Ready (M8)

### Aufgaben

- [ ] **8.1** Dockerfile optimieren
  - Multi-Stage Build
  - Minimales Basis-Image (distroless/alpine)
  - Non-root User
- [ ] **8.2** Docker Compose für lokale Entwicklung
  - Proxy + Redis
  - Optional: Prometheus + Grafana
- [ ] **8.3** Umfassende Dokumentation
  - Installation
  - Konfiguration
  - Troubleshooting
- [ ] **8.4** Integration Tests mit echten LLM-APIs (Mock)
- [ ] **8.5** Performance-Tests und Optimierung
- [ ] **8.6** Security Review
  - Keine Secrets in Logs
  - Sichere TLS-Konfiguration
  - Memory-Handling (Secrets nicht im Speicher halten)
- [ ] **8.7** Release v1.0.0

### Deliverables
- Production-ready Docker Image
- Vollständige Dokumentation
- v1.0.0 Release

---

## 🔮 Zukünftige Features (Backlog)

Diese Features sind für spätere Versionen geplant:

- [ ] **Regex-basierter Interceptor** – Konfigurierbare Patterns
- [ ] **HashiCorp Vault Integration** – Enterprise Secret Management
- [ ] **1Password Integration** – Weiterer Passwort-Manager
- [ ] **Web-UI Dashboard** – Optional für Monitoring
- [ ] **Kubernetes Helm Chart** – Einfaches K8s Deployment
- [ ] **Anthropic Protocol Handler** – Claude API Unterstützung
- [ ] **Ollama/Local LLM Support** – Lokale Modelle
- [ ] **Secret-Kategorisierung** – Verschiedene Ersetzungsstrategien pro Typ
- [ ] **Whitelist/Blacklist** – Bestimmte Domains/Patterns ignorieren

---

## 📝 Notizen

### Risiken

| Risiko | Wahrscheinlichkeit | Impact | Mitigation |
|--------|-------------------|--------|------------|
| TLS-Interception von Clients abgelehnt | Mittel | Hoch | Gute Doku für CA-Installation |
| Streaming-Buffer-Bugs | Mittel | Mittel | Umfassende Tests |
| Performance-Probleme bei vielen Secrets | Niedrig | Mittel | Profiling, Optimierung |
| Bitwarden API-Änderungen | Niedrig | Mittel | Version-Pinning, Monitoring |

### Abhängigkeiten

- Go 1.21+
- Redis (optional, für Multi-Instance)
- Docker (für Deployment)
- Bitwarden CLI (für Bitwarden-Interceptor)

---

## 📅 Timeline (Beispiel)

```
KW 1  ████████████████████ Phase 1: Projektstruktur
KW 2  ████████████████████ Phase 2: Proxy Core
KW 3  ████████████████████ Phase 3: Protokoll-Layer
KW 4  ████████████████████ Phase 4: Secret Interception
KW 5  ████████████████████ Phase 5: Mapping & Streaming
KW 6  ████████████████████ Phase 6: Observability
KW 7  ████████████████████ Phase 7: Bitwarden Integration
KW 8  ████████████████████ Phase 8: Production Ready
      ─────────────────────────────────────────────────►
                                              v1.0.0 🎉
```
