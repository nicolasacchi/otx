package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Project is a single named OneTrust tenant + credentials pair. ReadClientID
// and ReadClientSecret are the default credentials (used for read operations).
// WriteClientID and WriteClientSecret, when set, are used for any mutating
// command that also passes --confirm.
type Project struct {
	ClientID     string `toml:"client_id,omitempty"`
	ClientSecret string `toml:"client_secret,omitempty"`
	BaseURL      string `toml:"base_url,omitempty"`

	WriteClientID     string `toml:"write_client_id,omitempty"`
	WriteClientSecret string `toml:"write_client_secret,omitempty"`

	// APIKey is a long-lived OneTrust API key used as a direct bearer token,
	// an alternative to the OAuth client_id/client_secret pair.
	APIKey string `toml:"api_key,omitempty"`
}

type Config struct {
	DefaultProject string              `toml:"default_project,omitempty"`
	Projects       map[string]*Project `toml:"projects,omitempty"`
}

const defaultBaseURL = "https://app-eu.onetrust.com"

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "otx", "config.toml"), nil
}

// ConfigPath returns the path to the active config file (without checking it
// exists). Useful for `otx config current`.
func ConfigPath() string {
	p, _ := configFilePath()
	return p
}

func loadConfigFile() (*Config, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfigFile(cfg *Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func resolveProject(cfg *Config, projectFlag string) (string, *Project) {
	if cfg == nil {
		return "", nil
	}
	if projectFlag != "" {
		if p, ok := cfg.Projects[projectFlag]; ok {
			return projectFlag, p
		}
		return "", nil
	}
	if cfg.DefaultProject != "" {
		if p, ok := cfg.Projects[cfg.DefaultProject]; ok {
			return cfg.DefaultProject, p
		}
	}
	return "", nil
}

// Credentials holds the fully resolved credentials for the current call,
// already split by read vs write mode.
type Credentials struct {
	ProjectName  string
	ClientID     string
	ClientSecret string
	APIKey       string
	BaseURL      string
	WriteMode    bool
}

// AuthMode reports how the resolved credentials authenticate.
func (c *Credentials) AuthMode() string {
	if c.APIKey != "" {
		return "api_key"
	}
	return "oauth"
}

// LoadOptions controls which credential set LoadCredentials picks.
type LoadOptions struct {
	ClientIDFlag     string
	ClientSecretFlag string
	APIKeyFlag       string
	BaseURLFlag      string
	ProjectFlag      string
	WriteMode        bool // if true, prefer write_* fields / OTX_WRITE_CLIENT_* env vars
}

// LoadCredentials resolves credentials with precedence: flag > env > config.
// When WriteMode is true, write-scoped credentials are preferred (with fallback
// to the read pair, since some tenants share one credential set).
func LoadCredentials(opts LoadOptions) (*Credentials, error) {
	creds := &Credentials{WriteMode: opts.WriteMode}

	// Try flags first (apply to either mode).
	creds.ClientID = opts.ClientIDFlag
	creds.ClientSecret = opts.ClientSecretFlag
	creds.APIKey = opts.APIKeyFlag
	creds.BaseURL = opts.BaseURLFlag

	// Env vars.
	if creds.APIKey == "" {
		creds.APIKey = os.Getenv("OTX_API_KEY")
	}
	if opts.WriteMode {
		if creds.ClientID == "" {
			creds.ClientID = os.Getenv("OTX_WRITE_CLIENT_ID")
		}
		if creds.ClientSecret == "" {
			creds.ClientSecret = os.Getenv("OTX_WRITE_CLIENT_SECRET")
		}
	}
	if creds.ClientID == "" {
		creds.ClientID = os.Getenv("OTX_CLIENT_ID")
	}
	if creds.ClientSecret == "" {
		creds.ClientSecret = os.Getenv("OTX_CLIENT_SECRET")
	}
	if creds.BaseURL == "" {
		creds.BaseURL = os.Getenv("OTX_BASE_URL")
	}

	// Config file.
	if cfg, err := loadConfigFile(); err == nil {
		name, p := resolveProject(cfg, opts.ProjectFlag)
		if p != nil {
			creds.ProjectName = name
			if opts.WriteMode {
				if creds.ClientID == "" {
					creds.ClientID = firstNonEmpty(p.WriteClientID, p.ClientID)
				}
				if creds.ClientSecret == "" {
					creds.ClientSecret = firstNonEmpty(p.WriteClientSecret, p.ClientSecret)
				}
			} else {
				if creds.ClientID == "" {
					creds.ClientID = p.ClientID
				}
				if creds.ClientSecret == "" {
					creds.ClientSecret = p.ClientSecret
				}
			}
			if creds.APIKey == "" {
				creds.APIKey = p.APIKey
			}
			if creds.BaseURL == "" {
				creds.BaseURL = p.BaseURL
			}
		}
	}

	if creds.BaseURL == "" {
		creds.BaseURL = defaultBaseURL
	}
	// An API key is a complete, standalone credential — when present it bypasses
	// the OAuth client_id/client_secret requirement.
	if creds.APIKey != "" {
		return creds, nil
	}
	if creds.ClientID == "" {
		return nil, fmt.Errorf("client_id required: use --client-id / --api-key flag, OTX_CLIENT_ID / OTX_API_KEY env var, or run 'otx config add'")
	}
	if creds.ClientSecret == "" {
		return nil, fmt.Errorf("client_secret required: use --client-secret flag, OTX_CLIENT_SECRET env var, or run 'otx config add'")
	}
	return creds, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// AddProject persists a new project to the config file. If no default is set,
// the new project becomes the default.
func AddProject(name string, p *Project) error {
	cfg, err := loadConfigFile()
	if err != nil {
		cfg = &Config{}
	}
	if cfg.Projects == nil {
		cfg.Projects = make(map[string]*Project)
	}
	cfg.Projects[name] = p
	if cfg.DefaultProject == "" {
		cfg.DefaultProject = name
	}
	return saveConfigFile(cfg)
}

func RemoveProject(name string) error {
	cfg, err := loadConfigFile()
	if err != nil {
		return fmt.Errorf("no config file found")
	}
	if _, ok := cfg.Projects[name]; !ok {
		return fmt.Errorf("project %q not found", name)
	}
	delete(cfg.Projects, name)
	if cfg.DefaultProject == name {
		cfg.DefaultProject = ""
		for k := range cfg.Projects {
			cfg.DefaultProject = k
			break
		}
	}
	if len(cfg.Projects) == 0 {
		cfg.Projects = nil
	}
	return saveConfigFile(cfg)
}

func SetDefaultProject(name string) error {
	cfg, err := loadConfigFile()
	if err != nil {
		return fmt.Errorf("no config file found")
	}
	if _, ok := cfg.Projects[name]; !ok {
		return fmt.Errorf("project %q not found", name)
	}
	cfg.DefaultProject = name
	return saveConfigFile(cfg)
}

// ListProjects returns the parsed config (or nil if no file).
func ListProjects() (*Config, error) {
	return loadConfigFile()
}

// MaskSecret returns a redacted form of a secret string suitable for table
// display. The full value should never be printed.
func MaskSecret(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}
