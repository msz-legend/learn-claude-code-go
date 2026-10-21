package harness

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var dotenv = map[string]string{}

func init() {
	loadDotEnv()
}

func Workdir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func IsWindows() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") ||
		os.PathSeparator == '\\'
}

func EnvironmentPrompt() string {
	if IsWindows() {
		return "Windows: the bash tool runs through cmd.exe; use cmd.exe syntax, not Unix Bash " +
			"or PowerShell syntax, and prefer dedicated file tools for file operations"
	}
	return "Unix-like: the bash tool runs the system shell"
}

func Get(key string) string {
	if value, ok := dotenv[key]; ok && value != "" {
		return value
	}
	return os.Getenv(key)
}

func GetOr(key, fallback string) string {
	if value := Get(key); strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func Require(key string) string {
	value := Get(key)
	if strings.TrimSpace(value) == "" {
		panic("Missing " + key + ". Copy .env.example to .env and fill in your model settings.")
	}
	return value
}

func GetInt(key string, fallback int) int {
	value := strings.TrimSpace(Get(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func GetBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(Get(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func Protocol() string {
	if explicit := strings.TrimSpace(Get("LLM_PROTOCOL")); explicit != "" {
		return strings.ToLower(explicit)
	}
	if strings.TrimSpace(Get("ANTHROPIC_API_KEY")) != "" {
		return "anthropic"
	}
	if strings.TrimSpace(Get("OPENAI_API_KEY")) != "" {
		return "openai"
	}
	return "anthropic"
}

func Model() string {
	if model := strings.TrimSpace(Get("MODEL_ID")); model != "" {
		return model
	}
	if Protocol() == "openai" {
		return GetOr("OPENAI_MODEL", "gpt-4o-mini")
	}
	return GetOr("ANTHROPIC_MODEL", "claude-sonnet-4-5")
}

func BaseURL() string {
	value := GetOr("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	if Protocol() == "openai" {
		value = GetOr("OPENAI_BASE_URL", "https://api.openai.com/v1")
	}
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func MaxTokens() int {
	return GetInt("MAX_TOKENS", 8000)
}

func TimeoutSeconds() int {
	return GetInt("LLM_TIMEOUT_SECONDS", 180)
}

func DescribeModel() string {
	return Protocol() + " / " + Model() + " @ " + BaseURL()
}

func loadDotEnv() {
	file := filepath.Join(Workdir(), ".env")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		split := strings.Index(line, "=")
		if split <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:split])
		value := strings.TrimSpace(line[split+1:])
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		dotenv[key] = value
	}
}
