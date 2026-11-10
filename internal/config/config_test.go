package config

import (
"os"
"testing"
)

func TestLoad(t *testing.T) {
os.Setenv("ADDR", ":9090")
os.Setenv("LLM_MOCK", "true")
defer os.Unsetenv("ADDR")
defer os.Unsetenv("LLM_MOCK")

c := Load()
if c.Addr != ":9090" {
t.Fatalf("addr: %s", c.Addr)
}
if !c.LLMMock {
t.Fatal("mock must be true")
}
}

func TestLoadDotEnv(t *testing.T) {
f, _ := os.CreateTemp("", "*.env")
defer os.Remove(f.Name())
f.WriteString("FOO=bar\n# comment\nBAD\n")
f.Close()

os.Unsetenv("FOO")
LoadDotEnv(f.Name())
if os.Getenv("FOO") != "bar" {
t.Fatal("must load FOO")
}
os.Unsetenv("FOO")
}