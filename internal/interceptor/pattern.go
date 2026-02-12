package interceptor

import (
	"regexp"
)

// PatternRule defines a regex pattern for detecting secrets
type PatternRule struct {
	Name        string
	Pattern     *regexp.Regexp
	Type        string
	Confidence  float64
	Description string
}

// PatternInterceptor detects secrets using regex patterns
type PatternInterceptor struct {
	BaseInterceptor
	rules []PatternRule
}

// NewPatternInterceptor creates a new pattern-based interceptor with default rules
func NewPatternInterceptor() *PatternInterceptor {
	p := &PatternInterceptor{
		BaseInterceptor: BaseInterceptor{enabled: true},
		rules:           make([]PatternRule, 0),
	}

	// Add default patterns for common secret formats
	p.addDefaultRules()

	return p
}

// addDefaultRules adds commonly known secret patterns
func (p *PatternInterceptor) addDefaultRules() {
	defaultRules := []struct {
		name        string
		pattern     string
		secretType  string
		confidence  float64
		description string
	}{
		// OpenAI
		{
			name:        "openai_api_key",
			pattern:     `sk-[a-zA-Z0-9]{20,}T3BlbkFJ[a-zA-Z0-9]{20,}`,
			secretType:  "api_key",
			confidence:  1.0,
			description: "OpenAI API Key",
		},
		{
			name:        "openai_api_key_short",
			pattern:     `sk-[a-zA-Z0-9]{48,}`,
			secretType:  "api_key",
			confidence:  0.95,
			description: "OpenAI API Key (short format)",
		},
		// GitHub
		{
			name:        "github_token",
			pattern:     `ghp_[a-zA-Z0-9]{36}`,
			secretType:  "token",
			confidence:  1.0,
			description: "GitHub Personal Access Token",
		},
		{
			name:        "github_oauth",
			pattern:     `gho_[a-zA-Z0-9]{36}`,
			secretType:  "token",
			confidence:  1.0,
			description: "GitHub OAuth Access Token",
		},
		{
			name:        "github_app",
			pattern:     `ghu_[a-zA-Z0-9]{36}`,
			secretType:  "token",
			confidence:  1.0,
			description: "GitHub App User Token",
		},
		{
			name:        "github_refresh",
			pattern:     `ghr_[a-zA-Z0-9]{36}`,
			secretType:  "token",
			confidence:  1.0,
			description: "GitHub Refresh Token",
		},
		// AWS
		{
			name:        "aws_access_key",
			pattern:     `AKIA[0-9A-Z]{16}`,
			secretType:  "api_key",
			confidence:  1.0,
			description: "AWS Access Key ID",
		},
		{
			name:        "aws_secret_key",
			pattern:     `[0-9a-zA-Z/+]{40}`,
			secretType:  "api_key",
			confidence:  0.7, // Lower confidence, could be other base64
			description: "AWS Secret Access Key",
		},
		// Google
		{
			name:        "google_api_key",
			pattern:     `AIza[0-9A-Za-z\-_]{35}`,
			secretType:  "api_key",
			confidence:  1.0,
			description: "Google API Key",
		},
		// Slack
		{
			name:        "slack_token",
			pattern:     `xox[baprs]-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*`,
			secretType:  "token",
			confidence:  1.0,
			description: "Slack Token",
		},
		{
			name:        "slack_webhook",
			pattern:     `https://hooks\.slack\.com/services/T[a-zA-Z0-9_]{8}/B[a-zA-Z0-9_]{8,}/[a-zA-Z0-9_]{24}`,
			secretType:  "webhook",
			confidence:  1.0,
			description: "Slack Webhook URL",
		},
		// Stripe
		{
			name:        "stripe_live_key",
			pattern:     `sk_live_[0-9a-zA-Z]{24,}`,
			secretType:  "api_key",
			confidence:  1.0,
			description: "Stripe Live Secret Key",
		},
		{
			name:        "stripe_test_key",
			pattern:     `sk_test_[0-9a-zA-Z]{24,}`,
			secretType:  "api_key",
			confidence:  1.0,
			description: "Stripe Test Secret Key",
		},
		// Anthropic
		{
			name:        "anthropic_api_key",
			pattern:     `sk-ant-[a-zA-Z0-9\-]{32,}`,
			secretType:  "api_key",
			confidence:  1.0,
			description: "Anthropic API Key",
		},
		// Generic patterns
		{
			name:        "bearer_token",
			pattern:     `Bearer\s+[a-zA-Z0-9\-_\.]{20,}`,
			secretType:  "token",
			confidence:  0.9,
			description: "Bearer Token",
		},
		{
			name:        "basic_auth",
			pattern:     `Basic\s+[a-zA-Z0-9+/=]{20,}`,
			secretType:  "credentials",
			confidence:  0.9,
			description: "Basic Auth Credentials",
		},
		{
			name:        "private_key_header",
			pattern:     `-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,
			secretType:  "private_key",
			confidence:  1.0,
			description: "Private Key",
		},
		{
			name:        "password_assignment",
			pattern:     `(?i)(password|passwd|pwd|secret|token|api[_-]?key)\s*[:=]\s*['\"]?[a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':\"\\|,.<>\/?]{8,}['\"]?`,
			secretType:  "password",
			confidence:  0.85,
			description: "Password Assignment",
		},
		// Database connection strings
		{
			name:        "postgres_uri",
			pattern:     `postgres(?:ql)?://[^:]+:[^@]+@[^/]+/[^\s]+`,
			secretType:  "connection_string",
			confidence:  1.0,
			description: "PostgreSQL Connection String",
		},
		{
			name:        "mysql_uri",
			pattern:     `mysql://[^:]+:[^@]+@[^/]+/[^\s]+`,
			secretType:  "connection_string",
			confidence:  1.0,
			description: "MySQL Connection String",
		},
		{
			name:        "mongodb_uri",
			pattern:     `mongodb(\+srv)?://[^:]+:[^@]+@[^\s]+`,
			secretType:  "connection_string",
			confidence:  1.0,
			description: "MongoDB Connection String",
		},
		{
			name:        "redis_uri",
			pattern:     `redis://[^:]*:[^@]+@[^\s]+`,
			secretType:  "connection_string",
			confidence:  1.0,
			description: "Redis Connection String",
		},
	}

	for _, r := range defaultRules {
		compiled, err := regexp.Compile(r.pattern)
		if err != nil {
			continue // Skip invalid patterns
		}
		p.rules = append(p.rules, PatternRule{
			Name:        r.name,
			Pattern:     compiled,
			Type:        r.secretType,
			Confidence:  r.confidence,
			Description: r.description,
		})
	}
}

// Name returns the interceptor name
func (p *PatternInterceptor) Name() string {
	return "pattern"
}

// Configure applies configuration from config file
func (p *PatternInterceptor) Configure(config map[string]interface{}) error {
	// Add custom patterns from config
	if customPatterns, ok := config["patterns"].([]interface{}); ok {
		for _, cp := range customPatterns {
			if pattern, ok := cp.(map[string]interface{}); ok {
				name, _ := pattern["name"].(string)
				patternStr, _ := pattern["pattern"].(string)
				secretType, _ := pattern["type"].(string)
				confidence, _ := pattern["confidence"].(float64)

				if patternStr != "" {
					compiled, err := regexp.Compile(patternStr)
					if err != nil {
						continue
					}
					p.rules = append(p.rules, PatternRule{
						Name:       name,
						Pattern:    compiled,
						Type:       secretType,
						Confidence: confidence,
					})
				}
			}
		}
	}

	// Allow disabling specific rules
	if disabled, ok := config["disabled_rules"].([]interface{}); ok {
		disabledMap := make(map[string]bool)
		for _, d := range disabled {
			if name, ok := d.(string); ok {
				disabledMap[name] = true
			}
		}
		// Filter out disabled rules
		filtered := make([]PatternRule, 0)
		for _, rule := range p.rules {
			if !disabledMap[rule.Name] {
				filtered = append(filtered, rule)
			}
		}
		p.rules = filtered
	}

	return nil
}

// Detect analyzes text for pattern matches
func (p *PatternInterceptor) Detect(text string) []DetectedSecret {
	var secrets []DetectedSecret

	for _, rule := range p.rules {
		matches := rule.Pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			value := text[start:end]

			secrets = append(secrets, DetectedSecret{
				Value:      value,
				StartIndex: start,
				EndIndex:   end,
				Type:       rule.Type,
				Confidence: rule.Confidence,
			})
		}
	}

	return secrets
}

// AddRule adds a custom pattern rule
func (p *PatternInterceptor) AddRule(name, pattern, secretType string, confidence float64) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	p.rules = append(p.rules, PatternRule{
		Name:       name,
		Pattern:    compiled,
		Type:       secretType,
		Confidence: confidence,
	})

	return nil
}

// RuleCount returns the number of registered rules
func (p *PatternInterceptor) RuleCount() int {
	return len(p.rules)
}
