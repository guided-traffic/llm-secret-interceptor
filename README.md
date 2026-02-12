# LLM Secret Interceptor

Ein HTTPS-Proxy-Server, der Secrets in LLM-Kommunikation erkennt, maskiert und nach der Antwort wieder einsetzt. Entwickelt für die sichere Nutzung von LLM-Tools wie GitHub Copilot, ohne dass sensible Daten an Cloud-Provider übertragen werden.

## 🎯 Features

- **Man-in-the-Middle Proxy** mit eigener Self-Signed CA für TLS-Interception
- **Modulare Secret-Erkennung** durch Plugin-Architektur (Entropie-basiert, Bitwarden, erweiterbar)
- **Automatische Protokoll-Erkennung** für verschiedene LLM-APIs (OpenAI, Anthropic, etc.)
- **Streaming-Unterstützung** mit intelligentem Read-Ahead Buffer
- **Skalierbar** durch In-Memory oder Redis-basierte Mapping-Speicherung
- **Monitoring** via Prometheus-Metriken-Endpunkt

## 🏗️ Architektur

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           LLM Secret Interceptor                            │
└─────────────────────────────────────────────────────────────────────────────┘

┌──────────┐     HTTPS      ┌─────────────────────────────────────────────────────────────┐
│          │ ─────────────► │                      PROXY SERVER                           │
│  Client  │                │  ┌─────────────────────────────────────────────────────┐   │
│ (VSCode) │                │  │              TLS Interception Layer                 │   │
│          │ ◄───────────── │  │            (Self-Signed CA / MITM)                  │   │
└──────────┘     HTTPS      │  └─────────────────────────────────────────────────────┘   │
                            │                          │                                  │
                            │                          ▼                                  │
                            │  ┌─────────────────────────────────────────────────────┐   │
                            │  │           Protocol Auto-Detection                    │   │
                            │  │     (OpenAI Format, Anthropic Format, ...)          │   │
                            │  └─────────────────────────────────────────────────────┘   │
                            │                          │                                  │
                            │                          ▼                                  │
                            │  ┌─────────────────────────────────────────────────────┐   │
                            │  │         Standardized Internal Format                 │   │
                            │  └─────────────────────────────────────────────────────┘   │
                            │                          │                                  │
                            │                          ▼                                  │
                            │  ┌─────────────────────────────────────────────────────┐   │
                            │  │           Secret Interceptor Manager                 │   │
                            │  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐    │   │
                            │  │  │  Entropy    │ │  Bitwarden  │ │   Custom    │    │   │
                            │  │  │ Interceptor │ │ Interceptor │ │ Interceptor │    │   │
                            │  │  └─────────────┘ └─────────────┘ └─────────────┘    │   │
                            │  └─────────────────────────────────────────────────────┘   │
                            │                          │                                  │
                            │                          ▼                                  │
                            │  ┌─────────────────────────────────────────────────────┐   │
                            │  │              Secret Replacer                         │   │
                            │  │   "password123" → "__SECRET_a1b2c3d4__"             │   │
                            │  └─────────────────────────────────────────────────────┘   │
                            │                          │                                  │
                            │                          ▼                                  │
                            │  ┌─────────────────────────────────────────────────────┐   │
                            │  │           Mapping Store (TTL-based)                  │   │
                            │  │         [In-Memory Map] or [Redis]                   │   │
                            │  └─────────────────────────────────────────────────────┘   │
                            └──────────────────────────────────────────────────────────────┘
                                                       │
                                                       ▼ HTTPS
                            ┌──────────────────────────────────────────────────────────────┐
                            │                    LLM Cloud Provider                        │
                            │              (OpenAI, GitHub Copilot, etc.)                  │
                            └──────────────────────────────────────────────────────────────┘
```

## 🔄 Request/Response Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              REQUEST FLOW                                   │
└─────────────────────────────────────────────────────────────────────────────┘

  Client Request                    Proxy Processing                    To LLM
       │                                  │                                │
       │  "Fix bug with                   │                                │
       │   password: abc123"              │                                │
       │                                  │                                │
       ▼                                  ▼                                ▼
  ┌─────────┐    ┌─────────────┐    ┌───────────┐    ┌─────────────┐    ┌─────┐
  │ Client  │───►│ TLS Decrypt │───►│ Detect    │───►│ Replace     │───►│ LLM │
  │ Request │    │ & Parse     │    │ Secrets   │    │ Secrets     │    │ API │
  └─────────┘    └─────────────┘    └───────────┘    └─────────────┘    └─────┘
                                          │                │
                                          ▼                ▼
                                    ┌───────────┐    ┌───────────┐
                                    │ "abc123"  │    │ Store     │
                                    │ flagged   │    │ Mapping   │
                                    └───────────┘    └───────────┘

  Sent to LLM: "Fix bug with password: __SECRET_a1b2c3d4__"


┌─────────────────────────────────────────────────────────────────────────────┐
│                              RESPONSE FLOW                                  │
└─────────────────────────────────────────────────────────────────────────────┘

  From LLM                          Proxy Processing                  To Client
       │                                  │                                │
       │  "Change __SECRET_a1b2c3d4__     │                                │
       │   to a stronger password"        │                                │
       │                                  │                                │
       ▼                                  ▼                                ▼
  ┌─────────┐    ┌─────────────┐    ┌───────────┐    ┌─────────────┐    ┌────────┐
  │ LLM     │───►│ Read-Ahead  │───►│ Lookup    │───►│ Replace     │───►│ Client │
  │ Stream  │    │ Buffer      │    │ Mapping   │    │ Placeholder │    │        │
  └─────────┘    └─────────────┘    └───────────┘    └─────────────┘    └────────┘
                       │
                       ▼
                 ┌───────────┐
                 │ Buffer N  │
                 │ chars for │
                 │ streaming │
                 └───────────┘

  Sent to Client: "Change abc123 to a stronger password"
```

## 📦 Installation

### Docker (empfohlen)

```bash
docker pull ghcr.io/your-org/llm-secret-interceptor:latest

docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/certs:/app/certs \
  ghcr.io/your-org/llm-secret-interceptor:latest
```

### Build from Source

```bash
git clone https://github.com/your-org/llm-secret-interceptor.git
cd llm-secret-interceptor
go build -o llm-secret-interceptor ./cmd/proxy
```

## ⚙️ Konfiguration

Die Konfiguration erfolgt über eine YAML-Datei:

```yaml
# config.yaml
proxy:
  listen: ":8080"
  
tls:
  ca_cert: "/app/certs/ca.crt"
  ca_key: "/app/certs/ca.key"

storage:
  # "memory" für Single-Instance, "redis" für Multi-Instance
  type: "memory"
  redis:
    address: "localhost:6379"
    password: ""
    db: 0
  ttl: "24h"  # Mappings werden nach 24h Inaktivität gelöscht

placeholder:
  prefix: "__SECRET_"
  suffix: "__"
  
interceptors:
  entropy:
    enabled: true
    threshold: 4.5  # Shannon-Entropie Schwellenwert
    min_length: 8
    
  bitwarden:
    enabled: false
    server_url: "https://vault.bitwarden.com"
    # Credentials via Environment-Variablen

logging:
  level: "info"  # debug, info, warn, error
  audit:
    enabled: true
    log_interceptor_name: true
    log_secret_type: true
    # Secrets selbst werden NIEMALS geloggt!

metrics:
  enabled: true
  endpoint: "/metrics"
  port: 9090
```

## 🔧 VSCode Copilot Einrichtung

1. **CA-Zertifikat installieren:**
   ```bash
   # macOS
   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ./certs/ca.crt
   
   # Linux
   sudo cp ./certs/ca.crt /usr/local/share/ca-certificates/
   sudo update-ca-certificates
   ```

2. **Proxy in VSCode konfigurieren** (settings.json):
   ```json
   {
     "http.proxy": "https://localhost:8080",
     "http.proxyStrictSSL": true
   }
   ```

## 📊 Monitoring

### Prometheus Metriken

Der Proxy stellt folgende Metriken unter `/metrics` bereit:

- `llm_proxy_requests_total` – Gesamtanzahl verarbeiteter Requests
- `llm_proxy_secrets_detected_total` – Anzahl erkannter Secrets (nach Interceptor)
- `llm_proxy_secrets_replaced_total` – Anzahl ersetzter Secrets
- `llm_proxy_mapping_store_size` – Aktuelle Größe des Mapping-Stores
- `llm_proxy_request_duration_seconds` – Request-Latenz

## 🔌 Interceptor Plugin-System

Eigene Interceptors können implementiert werden:

```go
type SecretInterceptor interface {
    // Name returns the interceptor name for logging/metrics
    Name() string
    
    // Detect analyzes text and returns found secrets
    Detect(text string) []DetectedSecret
    
    // Configure applies configuration from YAML
    Configure(config map[string]interface{}) error
}

type DetectedSecret struct {
    Value      string
    StartIndex int
    EndIndex   int
    Type       string  // z.B. "password", "api_key", "token"
    Confidence float64 // 0.0 - 1.0
}
```

## 🛠️ Technologie-Stack

- **Sprache:** Go
- **TLS:** crypto/tls mit dynamischer Zertifikatsgenerierung
- **HTTP Proxy:** goproxy oder eigene Implementierung
- **Konfiguration:** gopkg.in/yaml.v3
- **Metriken:** prometheus/client_golang
- **Redis:** go-redis/redis

## 📄 Lizenz

Apache 2.0 License
