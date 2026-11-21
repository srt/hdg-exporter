package main

import "testing"

// Regression: every HDG_* env var must be bound in viper (via SetDefault), otherwise Unmarshal
// silently drops it. A missing HDG_WRITE_TOKEN made the write-enabled exporter log.Fatal on start.
func TestLoadConfigBindsAllWriteEnvVars(t *testing.T) {
	t.Setenv("HDG_ENDPOINT", "http://boiler")
	t.Setenv("HDG_WRITE_ENABLED", "true")
	t.Setenv("HDG_WRITE_TOKEN", "s3cret")
	t.Setenv("HDG_WRITE_ALLOW", `[{"id":6022,"type":"number"}]`)

	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !config.WriteEnabled {
		t.Error("WriteEnabled not bound from env")
	}
	if config.WriteToken != "s3cret" {
		t.Errorf("WriteToken not bound from env, got %q", config.WriteToken)
	}
	if config.WriteAllow == "" {
		t.Error("WriteAllow not bound from env")
	}
}
