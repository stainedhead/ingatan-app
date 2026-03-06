package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := LoadConfig("")
	require.NoError(t, err)

	assert.Equal(t, 8443, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "recursive", cfg.Chunking.Strategy)
	assert.Equal(t, 512, cfg.Chunking.ChunkSize)
	assert.Equal(t, 64, cfg.Chunking.ChunkOverlap)
	assert.Equal(t, 10485760, cfg.Chunking.MaxContentBytes)
	assert.Equal(t, 10485760, cfg.Ingest.MaxContentBytes)
	assert.True(t, cfg.Ingest.RobotsTxtCompliance)
	assert.Equal(t, 50, cfg.Conversation.AutoSummarizeMessageThreshold)
	assert.Equal(t, 8000, cfg.Conversation.AutoSummarizeTokenEstimateThreshold)
	assert.Equal(t, 300, cfg.RateLimit.RequestsPerMinute)
	assert.Equal(t, 50, cfg.RateLimit.BurstSize)
	assert.Equal(t, "ingatan", cfg.Telemetry.ServiceName)
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	t.Setenv("INGATAN_SERVER_PORT", "9000")
	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, 9000, cfg.Server.Port)
}

func TestLoadConfig_YAMLFile(t *testing.T) {
	yaml := `
server:
  port: 7777
  host: "127.0.0.1"
chunking:
  chunk_size: 256
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, 7777, cfg.Server.Port)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 256, cfg.Chunking.ChunkSize)
	// Non-overridden defaults remain.
	assert.Equal(t, 64, cfg.Chunking.ChunkOverlap)
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	assert.Error(t, err)
}
