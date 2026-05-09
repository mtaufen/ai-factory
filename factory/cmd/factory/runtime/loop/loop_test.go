package loop

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoopCmd_MissingConfig(t *testing.T) {
	// Set valid API key to test config flag
	os.Setenv("GEMINI_API_KEY", "dummy")
	defer os.Unsetenv("GEMINI_API_KEY")

	// Reset configPath to avoid pollution between tests
	configPath = ""

	cmd := Cmd
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --config flag")
	}
	if err.Error() != `required flag(s) "config" not set` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoopCmd_MissingAPIKey(t *testing.T) {
	os.Unsetenv("GEMINI_API_KEY")

	configPath = ""
	
	cmd := Cmd
	cmd.SetArgs([]string{"--config", "testdata/sample-run.yaml"})
	
	// Temporarily capture stdout/stderr or just check the error returned
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "GEMINI_API_KEY environment variable is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoopCmd_MissingManifest(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "dummy")
	defer os.Unsetenv("GEMINI_API_KEY")
	
	configPath = ""
	
	cmd := Cmd
	cmd.SetArgs([]string{"--config", "testdata/does-not-exist.yaml"})
	
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing manifest file")
	}
}

func TestLoopCmd_ValidSetup(t *testing.T) {
	// We won't actually execute the loop successfully without a real API key
	// or mock. The loop will attempt to initialize the real gemini model, 
	// which might fail if 'dummy' is not a real key but we can just check if 
	// initialization logic is sound until the model call.
	// Actually, `gemini.NewModel` doesn't make a network call on init if 
	// it just builds the client, so it might just fail at loop execution time,
	// which is what we want to verify (setup passes).
	
	os.Setenv("GEMINI_API_KEY", "dummy")
	defer os.Unsetenv("GEMINI_API_KEY")

	absPath, _ := filepath.Abs("testdata/sample-run.yaml")
	
	// Reset configPath
	configPath = ""

	cmd := Cmd
	cmd.SetArgs([]string{"--config", absPath})
	
	err := cmd.Execute()
	// It will likely fail with API key invalid at execution time, which means
	// flag parsing and manifest loading worked!
	if err != nil {
		// Just ensure it didn't fail for the early setup reasons
		if err.Error() == "GEMINI_API_KEY environment variable is required" || 
		   err.Error() == "--config flag is required" || 
		   err.Error() == "no Run resource found in manifests" {
			t.Fatalf("Setup failed early: %v", err)
		}
	}
}
