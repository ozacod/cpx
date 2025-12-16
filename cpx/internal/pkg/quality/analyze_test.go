package quality

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSummary(t *testing.T) {
	tests := []struct {
		name           string
		toolResults    ToolResults
		expectedTotal  int
		expectedByTool map[string]int
		expectedBySev  map[string]int
	}{
		{
			name: "Success status with results",
			toolResults: ToolResults{
				Tool:   "Cppcheck",
				Status: "success",
				Results: []AnalysisResult{
					{Severity: "warning", Message: "test1"},
					{Severity: "warning", Message: "test2"},
					{Severity: "error", Message: "test3"},
				},
			},
			expectedTotal:  3,
			expectedByTool: map[string]int{"Cppcheck": 3},
			expectedBySev:  map[string]int{"warning": 2, "error": 1},
		},
		{
			name: "Error status should not update summary",
			toolResults: ToolResults{
				Tool:   "clang-tidy",
				Status: "error",
				Results: []AnalysisResult{
					{Severity: "warning", Message: "test1"},
				},
				Error: "tool not found",
			},
			expectedTotal:  0,
			expectedByTool: map[string]int{},
			expectedBySev:  map[string]int{},
		},
		{
			name: "Empty results",
			toolResults: ToolResults{
				Tool:    "Flawfinder",
				Status:  "success",
				Results: []AnalysisResult{},
			},
			expectedTotal:  0,
			expectedByTool: map[string]int{"Flawfinder": 0},
			expectedBySev:  map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := ComprehensiveAnalysis{
				Timestamp: time.Now(),
				Tools:     []ToolResults{},
			}
			analysis.Summary.BySeverity = make(map[string]int)
			analysis.Summary.ByTool = make(map[string]int)

			updateSummary(&analysis, tt.toolResults)

			assert.Equal(t, tt.expectedTotal, analysis.Summary.TotalFindings)
			for tool, count := range tt.expectedByTool {
				assert.Equal(t, count, analysis.Summary.ByTool[tool])
			}
			for sev, count := range tt.expectedBySev {
				assert.Equal(t, count, analysis.Summary.BySeverity[sev])
			}
		})
	}
}

func TestExtractXMLAttr(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		attr     string
		expected string
	}{
		{
			name:     "Extract severity attribute",
			line:     `<error id="test" severity="warning" msg="test message">`,
			attr:     "severity",
			expected: "warning",
		},
		{
			name:     "Extract id attribute",
			line:     `<error id="unusedVariable" severity="style" msg="unused var">`,
			attr:     "id",
			expected: "unusedVariable",
		},
		{
			name:     "Extract msg attribute",
			line:     `<error id="test" msg="this is a test message" severity="error">`,
			attr:     "msg",
			expected: "this is a test message",
		},
		{
			name:     "Attribute not found",
			line:     `<error id="test" severity="warning">`,
			attr:     "nonexistent",
			expected: "",
		},
		{
			name:     "Empty line",
			line:     "",
			attr:     "severity",
			expected: "",
		},
		{
			name:     "Extract file attribute from location",
			line:     `<location file="src/main.cpp" line="10" column="5"/>`,
			attr:     "file",
			expected: "src/main.cpp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractXMLAttr(tt.line, tt.attr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractXMLInt(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		attr     string
		expected int
	}{
		{
			name:     "Extract line number",
			line:     `<location file="test.cpp" line="42" column="5"/>`,
			attr:     "line",
			expected: 42,
		},
		{
			name:     "Extract column number",
			line:     `<location file="test.cpp" line="10" column="15"/>`,
			attr:     "column",
			expected: 15,
		},
		{
			name:     "Attribute not found returns 0",
			line:     `<location file="test.cpp"/>`,
			attr:     "line",
			expected: 0,
		},
		{
			name:     "Non-numeric value returns 0",
			line:     `<location line="abc"/>`,
			attr:     "line",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractXMLInt(tt.line, tt.attr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseCSVLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "Simple CSV line",
			line:     "file.cpp,10,5,2,2,buffer,strcpy,Warning message",
			expected: []string{"file.cpp", "10", "5", "2", "2", "buffer", "strcpy", "Warning message"},
		},
		{
			name:     "CSV with quoted field containing comma",
			line:     `file.cpp,10,5,2,2,buffer,"Warning, be careful",suggestion`,
			expected: []string{"file.cpp", "10", "5", "2", "2", "buffer", "Warning, be careful", "suggestion"},
		},
		{
			name:     "CSV with escaped quotes",
			line:     `file.cpp,10,5,2,2,buffer,"Message with ""quotes""",note`,
			expected: []string{"file.cpp", "10", "5", "2", "2", "buffer", `Message with "quotes"`, "note"},
		},
		{
			name:     "Empty fields",
			line:     "file.cpp,,5,,,buffer,,",
			expected: []string{"file.cpp", "", "5", "", "", "buffer", "", ""},
		},
		{
			name:     "Single field",
			line:     "onlyfield",
			expected: []string{"onlyfield"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCSVLine(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseCppcheckErrorTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected []AnalysisResult
	}{
		{
			name: "Error tag with file0 and line fallback",
			tag:  `<error id="testId" severity="error" msg="Test message" file0="fallback.cpp" line="42"></error>`,
			expected: []AnalysisResult{
				{
					Tool:     "Cppcheck",
					Severity: "error",
					File:     "fallback.cpp",
					Line:     42,
					Message:  "Test message",
					Rule:     "testId",
				},
			},
		},
		{
			name: "Error tag with verbose message as fallback",
			tag:  `<error id="testId" severity="warning" verbose="Verbose message here" file0="test.cpp" line="5"></error>`,
			expected: []AnalysisResult{
				{
					Tool:     "Cppcheck",
					Severity: "warning",
					File:     "test.cpp",
					Line:     5,
					Message:  "Verbose message here",
					Rule:     "testId",
				},
			},
		},
		{
			name:     "Error tag with no file0 returns empty",
			tag:      `<error id="testId" severity="error" msg="No file"></error>`,
			expected: []AnalysisResult{},
		},
		{
			name:     "Error tag with file0 but no line returns empty",
			tag:      `<error id="testId" severity="error" msg="No line" file0="test.cpp"></error>`,
			expected: []AnalysisResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCppcheckErrorTag(tt.tag)
			assert.Equal(t, len(tt.expected), len(result))
			for i, exp := range tt.expected {
				if i < len(result) {
					assert.Equal(t, exp.Tool, result[i].Tool)
					assert.Equal(t, exp.Severity, result[i].Severity)
					assert.Equal(t, exp.File, result[i].File)
					assert.Equal(t, exp.Line, result[i].Line)
					assert.Equal(t, exp.Message, result[i].Message)
					assert.Equal(t, exp.Rule, result[i].Rule)
				}
			}
		})
	}
}

func TestParseCppcheckXML(t *testing.T) {
	// Create a temporary XML file
	tmpDir := t.TempDir()
	xmlFile := filepath.Join(tmpDir, "cppcheck.xml")

	// Use file0+line format which is the fallback parsing path
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<results version="2">
<cppcheck version="2.10"/>
<errors>
<error id="uninitvar" severity="error" msg="Uninitialized variable: ptr" file0="src/main.cpp" line="15"></error>
<error id="unusedFunction" severity="style" msg="Function helper is never used" file0="src/utils.cpp" line="42"></error>
</errors>
</results>`

	require.NoError(t, os.WriteFile(xmlFile, []byte(xmlContent), 0644))

	results := parseCppcheckXML(xmlFile)

	assert.Equal(t, 2, len(results))

	if len(results) >= 2 {
		// Check first result
		assert.Equal(t, "Cppcheck", results[0].Tool)
		assert.Equal(t, "error", results[0].Severity)
		assert.Equal(t, "src/main.cpp", results[0].File)
		assert.Equal(t, 15, results[0].Line)
		assert.Equal(t, "Uninitialized variable: ptr", results[0].Message)
		assert.Equal(t, "uninitvar", results[0].Rule)

		// Check second result
		assert.Equal(t, "Cppcheck", results[1].Tool)
		assert.Equal(t, "style", results[1].Severity)
		assert.Equal(t, "src/utils.cpp", results[1].File)
		assert.Equal(t, 42, results[1].Line)
	}
}

func TestParseCppcheckXML_NonExistentFile(t *testing.T) {
	results := parseCppcheckXML("/nonexistent/file.xml")
	assert.Empty(t, results)
}

func TestParseCppcheckXML_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	xmlFile := filepath.Join(tmpDir, "empty.xml")
	require.NoError(t, os.WriteFile(xmlFile, []byte(""), 0644))

	results := parseCppcheckXML(xmlFile)
	assert.Empty(t, results)
}

func TestParseClangTidyOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []AnalysisResult
	}{
		{
			name: "Single warning",
			output: `src/main.cpp:10:5: warning: unused variable 'x' [clang-diagnostic-unused-variable]
    int x = 5;
        ^`,
			expected: []AnalysisResult{
				{
					Tool:     "clang-tidy",
					Severity: "warning",
					File:     "src/main.cpp",
					Line:     10,
					Column:   5,
					Message:  "unused variable 'x'",
					Rule:     "clang-diagnostic-unused-variable",
				},
			},
		},
		{
			name: "Error with note",
			output: `src/main.cpp:20:10: error: use of undeclared identifier 'foo' [clang-diagnostic-error]
    foo();
         ^
src/main.cpp:5:1: note: did you mean 'bar'?`,
			expected: []AnalysisResult{
				{
					Tool:     "clang-tidy",
					Severity: "error",
					File:     "src/main.cpp",
					Line:     20,
					Column:   10,
					Message:  "use of undeclared identifier 'foo'; did you mean 'bar'?",
					Rule:     "clang-diagnostic-error",
				},
			},
		},
		{
			name: "Multiple warnings",
			output: `src/a.cpp:5:3: warning: message one [rule-one]
src/b.cpp:10:7: warning: message two [rule-two]`,
			expected: []AnalysisResult{
				{
					Tool:     "clang-tidy",
					Severity: "warning",
					File:     "src/a.cpp",
					Line:     5,
					Column:   3,
					Message:  "message one",
					Rule:     "rule-one",
				},
				{
					Tool:     "clang-tidy",
					Severity: "warning",
					File:     "src/b.cpp",
					Line:     10,
					Column:   7,
					Message:  "message two",
					Rule:     "rule-two",
				},
			},
		},
		{
			name:     "Empty output",
			output:   "",
			expected: []AnalysisResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := parseClangTidyOutput(tt.output)
			assert.Equal(t, len(tt.expected), len(results))
			for i, exp := range tt.expected {
				if i < len(results) {
					assert.Equal(t, exp.Tool, results[i].Tool)
					assert.Equal(t, exp.Severity, results[i].Severity)
					assert.Equal(t, exp.File, results[i].File)
					assert.Equal(t, exp.Line, results[i].Line)
					assert.Equal(t, exp.Rule, results[i].Rule)
				}
			}
		})
	}
}

func TestParseFlawfinderCSV(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []AnalysisResult
	}{
		{
			name:   "Standard CSV output",
			output: "File,Line,Column,DefaultLevel,Level,Category,Name,Warning,Suggestion\nsrc/main.cpp,10,5,2,3,buffer,strcpy,Does not check for buffer overflows,Consider using strncpy",
			expected: []AnalysisResult{
				{
					Tool:     "Flawfinder",
					Severity: "warning",
					File:     "src/main.cpp",
					Line:     10,
					Column:   5,
					Message:  "Does not check for buffer overflows. Consider using strncpy",
					Rule:     "buffer: strcpy",
				},
			},
		},
		{
			name:   "High severity (level 4+)",
			output: "src/main.cpp,20,1,4,5,format,printf,Format string vulnerability,Use snprintf instead",
			expected: []AnalysisResult{
				{
					Tool:     "Flawfinder",
					Severity: "error",
					File:     "src/main.cpp",
					Line:     20,
					Column:   1,
					Message:  "Format string vulnerability. Use snprintf instead",
					Rule:     "format: printf",
				},
			},
		},
		{
			name:   "Low severity (level 1)",
			output: "src/main.cpp,5,10,1,1,misc,getenv,Environment variable access,Be careful with env vars",
			expected: []AnalysisResult{
				{
					Tool:     "Flawfinder",
					Severity: "info",
					File:     "src/main.cpp",
					Line:     5,
					Column:   10,
					Message:  "Environment variable access. Be careful with env vars",
					Rule:     "misc: getenv",
				},
			},
		},
		{
			name:     "Empty output",
			output:   "",
			expected: []AnalysisResult{},
		},
		{
			name:     "Header only",
			output:   "File,Line,Column,DefaultLevel,Level,Category,Name,Warning",
			expected: []AnalysisResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := parseFlawfinderCSV(tt.output)
			assert.Equal(t, len(tt.expected), len(results))
			for i, exp := range tt.expected {
				if i < len(results) {
					assert.Equal(t, exp.Tool, results[i].Tool)
					assert.Equal(t, exp.Severity, results[i].Severity)
					assert.Equal(t, exp.File, results[i].File)
					assert.Equal(t, exp.Line, results[i].Line)
					assert.Equal(t, exp.Rule, results[i].Rule)
				}
			}
		})
	}
}

func TestDiscoverSourceDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create common source directories
	require.NoError(t, os.MkdirAll("src", 0755))
	require.NoError(t, os.MkdirAll("include", 0755))
	require.NoError(t, os.MkdirAll("build", 0755)) // Should be skipped

	// Create a C++ file in src
	require.NoError(t, os.WriteFile("src/main.cpp", []byte("int main() {}"), 0644))

	tests := []struct {
		name     string
		targets  []string
		expected []string
	}{
		{
			name:    "Discover with default targets",
			targets: []string{"."},
			// When targets is ".", it tries to discover common directories
			// But since we're not in a git repo, hasCppFiles returns true
		},
		{
			name:    "Skip build directory",
			targets: []string{"build"},
			// build is in skipDirs, so should return empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirs := discoverSourceDirectories(tt.targets)
			// Just verify it doesn't panic and returns a slice
			assert.NotNil(t, dirs)

			// Verify build directory is never included
			for _, dir := range dirs {
				assert.NotEqual(t, "build", dir)
				assert.NotEqual(t, "builddir", dir)
				assert.NotEqual(t, ".bazel", dir)
			}
		})
	}
}

func TestGenerateHTMLReport(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "report.html")

	analysis := ComprehensiveAnalysis{
		Timestamp: time.Now(),
		Tools: []ToolResults{
			{
				Tool:   "Cppcheck",
				Status: "success",
				Results: []AnalysisResult{
					{
						Tool:     "Cppcheck",
						Severity: "warning",
						File:     "src/main.cpp",
						Line:     10,
						Column:   5,
						Message:  "Test warning",
						Rule:     "testRule",
					},
				},
			},
		},
	}
	analysis.Summary.TotalFindings = 1
	analysis.Summary.BySeverity = map[string]int{"warning": 1}
	analysis.Summary.ByTool = map[string]int{"Cppcheck": 1}

	err := generateHTMLReport(analysis, outputFile)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(outputFile)
	require.NoError(t, err)

	// Read and verify content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	assert.Contains(t, string(content), "Cpx Code Analysis Report")
	assert.Contains(t, string(content), "Cppcheck")
	assert.Contains(t, string(content), "src/main.cpp")
	assert.Contains(t, string(content), "Test warning")
}

func TestAnalysisResultStruct(t *testing.T) {
	result := AnalysisResult{
		Tool:      "Cppcheck",
		Severity:  "error",
		File:      "test.cpp",
		Line:      10,
		Column:    5,
		Message:   "Test message",
		Rule:      "testRule",
		Code:      "int x;",
		EndLine:   12,
		EndColumn: 10,
	}

	assert.Equal(t, "Cppcheck", result.Tool)
	assert.Equal(t, "error", result.Severity)
	assert.Equal(t, "test.cpp", result.File)
	assert.Equal(t, 10, result.Line)
	assert.Equal(t, 5, result.Column)
	assert.Equal(t, "Test message", result.Message)
	assert.Equal(t, "testRule", result.Rule)
	assert.Equal(t, "int x;", result.Code)
	assert.Equal(t, 12, result.EndLine)
	assert.Equal(t, 10, result.EndColumn)
}

func TestToolResultsStruct(t *testing.T) {
	results := ToolResults{
		Tool:   "clang-tidy",
		Status: "success",
		Results: []AnalysisResult{
			{Tool: "clang-tidy", Message: "test"},
		},
		Error: "",
	}

	assert.Equal(t, "clang-tidy", results.Tool)
	assert.Equal(t, "success", results.Status)
	assert.Equal(t, 1, len(results.Results))
	assert.Empty(t, results.Error)
}
