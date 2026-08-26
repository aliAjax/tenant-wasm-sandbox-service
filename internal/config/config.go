package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTP     HTTP     `yaml:"http"`
	GRPC     GRPC     `yaml:"grpc"`
	Runtime  Runtime  `yaml:"runtime"`
	Tenant   Tenant   `yaml:"tenant"`
	Storage  Storage  `yaml:"storage"`
	Security Security `yaml:"security"`
	Worker   Worker   `yaml:"worker"`
	Node     Node     `yaml:"node"`
}
type HTTP struct {
	Address        string        `yaml:"address"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	MaxBodyBytes   int64         `yaml:"max_body_bytes"`
	RatePerSecond  int           `yaml:"rate_per_second"`
}
type GRPC struct {
	Address string `yaml:"address"`
}
type Runtime struct {
	Version          string        `yaml:"version"`
	MaxModuleBytes   int64         `yaml:"max_module_bytes"`
	MaxInstructions  uint64        `yaml:"max_instructions"`
	MaxMemoryPages   uint32        `yaml:"max_memory_pages"`
	MaxStackBytes    uint64        `yaml:"max_stack_bytes"`
	MaxExecutionTime time.Duration `yaml:"max_execution_time"`
	MaxOutputBytes   int           `yaml:"max_output_bytes"`
	CacheTTL         time.Duration `yaml:"cache_ttl"`
}
type Tenant struct {
	MaxConcurrent   int           `yaml:"max_concurrent"`
	MaxCPUPerMinute time.Duration `yaml:"max_cpu_per_minute"`
	MaxMemoryPages  uint32        `yaml:"max_memory_pages"`
}
type Storage struct {
	StateFile string `yaml:"state_file"`
}
type Security struct {
	APIKey               string `yaml:"api_key"`
	AllowUnsignedModules bool   `yaml:"allow_unsigned_modules"`
}
type Worker struct {
	Count int `yaml:"count"`
}
type Node struct {
	ID string `yaml:"id"`
}

func Default() Config {
	return Config{HTTP: HTTP{Address: ":8080", ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, RequestTimeout: 5 * time.Second, MaxBodyBytes: 2 << 20, RatePerSecond: 100}, GRPC: GRPC{Address: ":9090"}, Runtime: Runtime{Version: "sim/v1", MaxModuleBytes: 1 << 20, MaxInstructions: 1000000, MaxMemoryPages: 256, MaxStackBytes: 2 << 20, MaxExecutionTime: 5 * time.Second, MaxOutputBytes: 128 << 10, CacheTTL: 15 * time.Minute}, Tenant: Tenant{MaxConcurrent: 2, MaxCPUPerMinute: 30 * time.Second, MaxMemoryPages: 256}, Storage: Storage{StateFile: "data/state.json"}, Security: Security{APIKey: "dev-secret", AllowUnsignedModules: true}, Worker: Worker{Count: 2}, Node: Node{ID: "node-1"}}
}
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		if err = yaml.Unmarshal(b, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
	}
	applyEnv(&cfg)
	return cfg, validate(cfg)
}
func applyEnv(c *Config) {
	setString("WASM_HTTP_ADDRESS", &c.HTTP.Address)
	setString("WASM_GRPC_ADDRESS", &c.GRPC.Address)
	setString("WASM_STATE_FILE", &c.Storage.StateFile)
	setString("WASM_API_KEY", &c.Security.APIKey)
	setString("WASM_NODE_ID", &c.Node.ID)
	setInt("WASM_WORKERS", &c.Worker.Count)
	setBool("WASM_ALLOW_UNSIGNED", &c.Security.AllowUnsignedModules)
}
func setString(key string, dst *string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = v
	}
}
func setInt(key string, dst *int) {
	if v, ok := os.LookupEnv(key); ok {
		if n, e := strconv.Atoi(v); e == nil {
			*dst = n
		}
	}
}
func setBool(key string, dst *bool) {
	if v, ok := os.LookupEnv(key); ok {
		if b, e := strconv.ParseBool(v); e == nil {
			*dst = b
		}
	}
}
func validate(c Config) error {
	var problems []string
	if c.HTTP.Address == "" {
		problems = append(problems, "http.address is required")
	}
	if c.GRPC.Address == "" {
		problems = append(problems, "grpc.address is required")
	}
	if c.Worker.Count < 1 {
		problems = append(problems, "worker.count must be positive")
	}
	if c.Runtime.MaxModuleBytes < 1 {
		problems = append(problems, "runtime.max_module_bytes must be positive")
	}
	if c.Security.APIKey == "" {
		problems = append(problems, "security.api_key is required")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(problems, ", "))
	}
	return nil
}
