package ide

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ozacod/cpx/internal/build/common"
	"github.com/ozacod/cpx/internal/utils"
)

type ZedGenerator struct {
	DebugArgs []string
	EnvVars   map[string]string
}

func NewZedGenerator(debugArgs []string, envVars map[string]string) *ZedGenerator {
	if debugArgs == nil {
		debugArgs = []string{}
	}
	if envVars == nil {
		envVars = map[string]string{}
	}
	return &ZedGenerator{
		DebugArgs: debugArgs,
		EnvVars:   envVars,
	}
}

func (z *ZedGenerator) Generate() error {
	zedDir := ".zed"
	if err := os.MkdirAll(zedDir, 0755); err != nil {
		return fmt.Errorf("failed to create .zed directory: %w", err)
	}

	if err := z.generateSettings(zedDir); err != nil {
		return err
	}

	if err := z.generateTasks(zedDir); err != nil {
		return err
	}

	if err := z.generateKeymap(zedDir); err != nil {
		return err
	}

	if err := z.generateLaunch(zedDir); err != nil {
		return err
	}

	utils.PrintSuccess("Generated Zed configuration in .zed/")
	return nil
}

type ZedLaunch struct {
	Label       string   `json:"label"`
	Adapter     string   `json:"adapter"`
	Request     string   `json:"request"`
	Program     string   `json:"program"`
	Args        []string `json:"args"`
	Cwd         string   `json:"cwd"`
	StopOnEntry bool     `json:"stopOnEntry"`
}

func (z *ZedGenerator) generateLaunch(dir string) error {
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	// Default debug path: .bin/native/debug/<project_name>
	// Note: We use .bin/native/debug because that's where cpx puts debug binaries
	programPath := fmt.Sprintf("${ZED_WORKTREE_ROOT}/.bin/native/debug/%s", projectName)

	launch := []ZedLaunch{
		{
			Label:       "Debug " + projectName,
			Adapter:     "CodeLLDB",
			Request:     "launch",
			Program:     programPath,
			Args:        z.DebugArgs,
			Cwd:         "${ZED_WORKTREE_ROOT}",
			StopOnEntry: false,
		},
	}

	return writeJSON(filepath.Join(dir, "debug.json"), launch)
}

func (z *ZedGenerator) generateKeymap(dir string) error {
	// Define the bindings we want to inject
	cpxBindings := map[string]interface{}{
		"ctrl-cmd-m": []interface{}{"task::Spawn", map[string]string{"task_name": "build"}},
		"ctrl-cmd-k": []interface{}{"task::Spawn", map[string]string{"task_name": "clean"}},
		"ctrl-cmd-r": []interface{}{"task::Spawn", map[string]string{"task_name": "run"}},
	}

	// Try to inject into global keymap
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalKeymapPath := filepath.Join(homeDir, ".config", "zed", "keymap.json")
		if err := z.injectBindingsToGlobalKeymap(globalKeymapPath, cpxBindings); err != nil {
			utils.PrintWarning("Could not inject bindings to global keymap: %v", err)
		} else {
			utils.PrintSuccess("Injected CPX keybindings into %s", globalKeymapPath)
			return nil
		}
	}

	// Fallback: write recommended file
	keymap := []map[string]interface{}{
		{
			"context":  "Workspace",
			"bindings": cpxBindings,
		},
	}

	content, err := json.MarshalIndent(keymap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json for keymap: %w", err)
	}

	// Post-process to compact task::Spawn bindings
	re := regexp.MustCompile(`\[\s*"task::Spawn",\s*\{\s*"task_name":\s*"([^"]+)"\s*\}\s*\]`)
	compacted := re.ReplaceAll(content, []byte(`["task::Spawn", {"task_name": "$1"}]`))

	compacted = addTrailingCommas(compacted)

	path := filepath.Join(dir, "recommended_keymap.json")
	if _, err := os.Stat(path); err == nil {
		utils.PrintWarning("File %s already exists, skipping...", path)
		return nil
	}

	if err := os.WriteFile(path, compacted, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	utils.PrintInfo("Created %s (Action Required: Copy these bindings to ~/.config/zed/keymap.json)", path)
	return nil
}

func (z *ZedGenerator) injectBindingsToGlobalKeymap(path string, bindings map[string]interface{}) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var existingKeymap []map[string]interface{}

	// Read existing keymap if it exists
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		// Remove trailing commas for parsing (Zed uses JSON5-like format)
		cleanData := regexp.MustCompile(`,\s*([}\]])`).ReplaceAll(data, []byte("$1"))
		// Skip if still empty after cleaning
		if len(cleanData) > 0 {
			if err := json.Unmarshal(cleanData, &existingKeymap); err != nil {
				return fmt.Errorf("failed to parse existing keymap: %w", err)
			}
		}
	}

	// Check if we already have a CPX Workspace context
	found := false
	for i, entry := range existingKeymap {
		ctx, hasCtx := entry["context"].(string)
		if hasCtx && ctx == "Workspace" {
			// Check if our bindings are already there
			if existingBindings, ok := entry["bindings"].(map[string]interface{}); ok {
				allPresent := true
				for key := range bindings {
					if _, exists := existingBindings[key]; !exists {
						allPresent = false
						break
					}
				}
				if allPresent {
					// Already injected
					return nil
				}
				// Merge bindings
				for key, val := range bindings {
					existingBindings[key] = val
				}
				existingKeymap[i]["bindings"] = existingBindings
				found = true
				break
			}
		}
	}

	if !found {
		// Add new entry
		existingKeymap = append(existingKeymap, map[string]interface{}{
			"context":  "Workspace",
			"bindings": bindings,
		})
	}

	// Marshal back
	content, err := json.MarshalIndent(existingKeymap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keymap: %w", err)
	}

	// Post-process to compact task::Spawn bindings
	re := regexp.MustCompile(`\[\s*"task::Spawn",\s*\{\s*"task_name":\s*"([^"]+)"\s*\}\s*\]`)
	compacted := re.ReplaceAll(content, []byte(`["task::Spawn", {"task_name": "$1"}]`))

	compacted = addTrailingCommas(compacted)

	if err := os.WriteFile(path, compacted, 0644); err != nil {
		return fmt.Errorf("failed to write keymap: %w", err)
	}

	return nil
}

func (z *ZedGenerator) generateSettings(dir string) error {
	settings := map[string]interface{}{
		"languages": map[string]interface{}{
			"C++": map[string]interface{}{
				"format_on_save": "on",
				"formatter":      "language_server",
			},
		},
		"lsp": map[string]interface{}{
			"clangd": map[string]interface{}{
				"binary": map[string]interface{}{
					"arguments": []string{
						"--compile-commands-dir=.cache/native/debug",
						"--background-index",
						"--clang-tidy",
						"--header-insertion=iwyu",
						"--completion-style=detailed",
					},
				},
			},
		},
	}

	return writeJSON(filepath.Join(dir, "settings.json"), settings)
}

type ZedTask struct {
	Label   string            `json:"label"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

func (z *ZedGenerator) generateTasks(dir string) error {
	tasks := []ZedTask{
		{
			Label:   "build",
			Command: "cpx build",
			Args:    []string{},
			Env:     map[string]string{},
		},
		{
			Label:   "build (release)",
			Command: "cpx build --release",
			Args:    []string{},
			Env:     map[string]string{},
		},
		{
			Label:   "run",
			Command: "cpx run",
			Args:    z.DebugArgs,
			Env:     z.EnvVars,
		},
		{
			Label:   "run (release)",
			Command: "cpx run --release",
			Args:    z.DebugArgs,
			Env:     z.EnvVars,
		},
		{
			Label:   "test",
			Command: "cpx test",
			Args:    []string{},
			Env:     map[string]string{},
		},
		{
			Label:   "bench",
			Command: "cpx bench",
			Args:    []string{},
			Env:     map[string]string{},
		},
		{
			Label:   "clean",
			Command: "cpx clean",
			Args:    []string{},
			Env:     map[string]string{},
		},
	}

	return writeJSON(filepath.Join(dir, "tasks.json"), tasks)
}

func writeJSON(path string, data interface{}) error {
	// Don't overwrite if exists
	if _, err := os.Stat(path); err == nil {
		utils.PrintWarning("File %s already exists, skipping...", path)
		return nil
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json for %s: %w", path, err)
	}

	content = addTrailingCommas(content)

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	utils.PrintInfo("Created %s", path)
	return nil
}

func addTrailingCommas(content []byte) []byte {
	// Add trailing commas to the last element of objects and arrays
	// This regex looks for a non, non-[ character followed by a newline and a closing brace/bracket
	// and inserts a comma.
	re := regexp.MustCompile(`([^\[{])(\n\s*[}\]])`)
	return re.ReplaceAll(content, []byte("$1,$2"))
}
