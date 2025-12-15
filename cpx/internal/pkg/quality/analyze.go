package quality

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AnalysisResult represents a single finding from any tool
type AnalysisResult struct {
	Tool      string `json:"tool"`
	Severity  string `json:"severity"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Message   string `json:"message"`
	Rule      string `json:"rule,omitempty"`
	Code      string `json:"code,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
}

// ToolResults contains all results from a single tool
type ToolResults struct {
	Tool    string           `json:"tool"`
	Status  string           `json:"status"`
	Results []AnalysisResult `json:"results"`
	Error   string           `json:"error,omitempty"`
}

// ComprehensiveAnalysis contains all results from all tools
type ComprehensiveAnalysis struct {
	Timestamp time.Time     `json:"timestamp"`
	Tools     []ToolResults `json:"tools"`
	Summary   struct {
		TotalFindings int            `json:"total_findings"`
		BySeverity    map[string]int `json:"by_severity"`
		ByTool        map[string]int `json:"by_tool"`
	} `json:"summary"`
}

// RunComprehensiveAnalysis runs all analysis tools and generates an HTML report
func RunComprehensiveAnalysis(outputFile string, skipCppcheck, skipLint, skipFlawfinder bool, targets []string, vcpkg VcpkgSetup) error {
	fmt.Printf("%sRunning comprehensive code analysis...%s\n", Cyan, Reset)

	analysis := ComprehensiveAnalysis{
		Timestamp: time.Now(),
		Tools:     []ToolResults{},
	}
	analysis.Summary.BySeverity = make(map[string]int)
	analysis.Summary.ByTool = make(map[string]int)

	// Run Cppcheck
	if !skipCppcheck {
		fmt.Printf("%sRunning Cppcheck...%s\n", Cyan, Reset)
		cppcheckResults := runCppcheckAnalysis(targets)
		analysis.Tools = append(analysis.Tools, cppcheckResults)
		updateSummary(&analysis, cppcheckResults)
	}

	// Run clang-tidy
	if !skipLint {
		fmt.Printf("%sRunning clang-tidy...%s\n", Cyan, Reset)
		lintResults := runLintAnalysis(vcpkg)
		analysis.Tools = append(analysis.Tools, lintResults)
		updateSummary(&analysis, lintResults)
	}

	// Run Flawfinder
	if !skipFlawfinder {
		fmt.Printf("%sRunning Flawfinder...%s\n", Cyan, Reset)
		flawfinderResults := runFlawfinderAnalysis(targets)
		analysis.Tools = append(analysis.Tools, flawfinderResults)
		updateSummary(&analysis, flawfinderResults)
	}

	// Generate HTML report
	fmt.Printf("%sGenerating HTML report...%s\n", Cyan, Reset)
	if err := generateHTMLReport(analysis, outputFile); err != nil {
		return fmt.Errorf("failed to generate HTML report: %w", err)
	}

	fmt.Printf("%sAnalysis complete! Report saved to: %s%s\n", Green, outputFile, Reset)
	fmt.Printf("   Total findings: %d\n", analysis.Summary.TotalFindings)
	for tool, count := range analysis.Summary.ByTool {
		fmt.Printf("   %s: %d findings\n", tool, count)
	}

	return nil
}

func updateSummary(analysis *ComprehensiveAnalysis, toolResults ToolResults) {
	if toolResults.Status == "error" {
		return
	}

	count := len(toolResults.Results)
	analysis.Summary.TotalFindings += count
	analysis.Summary.ByTool[toolResults.Tool] = count

	for _, result := range toolResults.Results {
		analysis.Summary.BySeverity[result.Severity]++
	}
}

// discoverSourceDirectories finds source directories to scan
// Looks for common directories like src/, include/, lib/, etc.
// Respects .gitignore by checking if directories contain git-tracked files
func discoverSourceDirectories(targets []string) []string {
	var dirs []string

	// Common source directory names
	commonDirs := []string{"src", "examples", "include", "lib", "libs", "source", "sources", "test", "tests"}

	// If specific targets are provided and they're directories, use them
	if len(targets) > 0 && targets[0] != "." {
		for _, target := range targets {
			if info, err := os.Stat(target); err == nil && info.IsDir() {
				// Check if directory contains C/C++ files (respecting .gitignore)
				if hasCppFiles(target) {
					dirs = append(dirs, target)
				}
			}
		}
		if len(dirs) > 0 {
			return dirs
		}
	}

	// Otherwise, discover common source directories
	for _, dirName := range commonDirs {
		if info, err := os.Stat(dirName); err == nil && info.IsDir() {
			// Check if directory contains C/C++ files (respecting .gitignore)
			if hasCppFiles(dirName) {
				dirs = append(dirs, dirName)
			}
		}
	}

	// If no common directories found, check current directory
	if len(dirs) == 0 {
		if hasCppFiles(".") {
			dirs = append(dirs, ".")
		}
	}

	return dirs
}

// hasCppFiles checks if a directory contains C/C++ files
// Uses git-tracked files to respect .gitignore
func hasCppFiles(dir string) bool {
	// Get git-tracked C/C++ files
	trackedFiles, err := GetGitTrackedCppFiles()
	if err != nil {
		// If not in git repo, assume directory has files if it exists
		// cppcheck will handle scanning and respecting ignore patterns
		return true
	}

	// Check if any tracked files are in this directory
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	// Normalize the directory path (ensure it ends with separator for prefix check)
	dirWithSep := absDir + string(filepath.Separator)

	for _, file := range trackedFiles {
		absFile, err := filepath.Abs(file)
		if err != nil {
			continue
		}
		// Check if file is in the directory
		if strings.HasPrefix(absFile, dirWithSep) || absFile == absDir {
			return true
		}
	}

	return false
}

func runCppcheckAnalysis(targets []string) ToolResults {
	result := ToolResults{
		Tool:    "Cppcheck",
		Status:  "success",
		Results: []AnalysisResult{},
	}

	// Check if cppcheck is available
	if _, err := exec.LookPath("cppcheck"); err != nil {
		result.Status = "skipped"
		result.Error = "cppcheck not found"
		return result
	}

	// Discover source directories to scan
	// Look for common source directories like src/, include/, lib/, etc.
	sourceDirs := discoverSourceDirectories(targets)
	if len(sourceDirs) == 0 {
		result.Status = "skipped"
		result.Error = "no source directories found to scan"
		if os.Getenv("CPX_DEBUG") != "" {
			fmt.Printf("Debug: cppcheck no source directories found, targets: %v\n", targets)
		}
		return result
	}

	if os.Getenv("CPX_DEBUG") != "" {
		fmt.Printf("Debug: cppcheck targets: %v\n", targets)
		fmt.Printf("Debug: cppcheck sourceDirs: %v\n", sourceDirs)
	}

	// Create temporary XML file
	tmpXML, err := os.CreateTemp("", "cppcheck-*.xml")
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("failed to create temp file: %v", err)
		return result
	}
	defer os.Remove(tmpXML.Name())
	tmpXML.Close()

	// Run cppcheck with XML output directly to file
	// Using --xml with --output-file writes XML directly to the file
	// Pass directories to scan (cppcheck will scan all non-ignored files in those directories)
	args := []string{"--enable=all", "--xml", "--xml-version=2", "--output-file=" + tmpXML.Name()}
	args = append(args, sourceDirs...)

	// Run cppcheck - XML will be written directly to the file
	cmd := exec.Command("cppcheck", args...)
	// Suppress stdout (progress messages) and stderr (errors)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	nullFile, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if nullFile != nil {
		cmd.Stdout = nullFile
		defer nullFile.Close()
	}
	_ = cmd.Run()

	// Check if XML file was written and has content
	fileInfo, err := os.Stat(tmpXML.Name())
	if err != nil || (fileInfo != nil && fileInfo.Size() == 0) {
		// File is empty or doesn't exist, try to get XML from stderr as fallback
		if stderr.Len() > 0 {
			stderrStr := stderr.String()
			// Find XML start in stderr
			xmlStart := strings.Index(stderrStr, "<?xml")
			if xmlStart == -1 {
				xmlStart = strings.Index(stderrStr, "<results>")
			}
			if xmlStart == -1 {
				xmlStart = strings.Index(stderrStr, "<error")
			}
			if xmlStart >= 0 {
				xmlContent := stderrStr[xmlStart:]
				xmlEnd := strings.LastIndex(xmlContent, "</results>")
				if xmlEnd > 0 {
					xmlContent = xmlContent[:xmlEnd+9]
				}
				_ = os.WriteFile(tmpXML.Name(), []byte(xmlContent), 0644)
			}
		}
	}

	// Parse XML output from the file
	results := parseCppcheckXML(tmpXML.Name())
	result.Results = results

	return result
}

func parseCppcheckXML(xmlFile string) []AnalysisResult {
	results := []AnalysisResult{}

	// Read XML file
	data, err := os.ReadFile(xmlFile)
	if err != nil {
		return results
	}

	// Parse XML - cppcheck XML format has <error> tags with <location> tags inside
	content := string(data)
	errorTagCount := 0

	// Find all <error> tags (they can span multiple lines)
	// Use strings.Index to find all occurrences
	searchStart := 0
	for {
		// Find next <error tag
		errorStart := strings.Index(content[searchStart:], "<error")
		if errorStart == -1 {
			break
		}
		errorStart += searchStart

		// Find the closing </error> tag
		errorEnd := strings.Index(content[errorStart:], "</error>")
		if errorEnd == -1 {
			// No closing tag found, skip this one
			searchStart = errorStart + 7
			continue
		}
		errorEnd += errorStart + 8 // +8 for "</error>" length

		errorTag := content[errorStart:errorEnd]
		errorTagCount++

		// Parse error tag and extract all locations
		errorResults := parseCppcheckErrorTag(errorTag)
		results = append(results, errorResults...)

		// Move search start past this error tag
		searchStart = errorEnd
	}

	return results
}

func parseCppcheckErrorTag(tag string) []AnalysisResult {
	results := []AnalysisResult{}

	// Extract attributes from the error tag
	severity := extractXMLAttr(tag, "severity")
	msg := extractXMLAttr(tag, "msg")
	if msg == "" {
		// Try verbose attribute
		msg = extractXMLAttr(tag, "verbose")
	}
	id := extractXMLAttr(tag, "id")

	// Extract file0 attribute as fallback
	file0 := extractXMLAttr(tag, "file0")

	locationCount := 0

	// Find all <location> tags inside the error tag
	// Pattern: <location file="..." line="..." column="..." .../>
	i := 0
	for i < len(tag) {
		if i+10 < len(tag) && tag[i:i+10] == "<location" {
			locationStart := i
			// Find closing of location tag
			locationEnd := -1
			for j := i + 10; j < len(tag); j++ {
				if j+2 <= len(tag) && tag[j:j+2] == "/>" {
					locationEnd = j + 2
					break
				}
				if j+11 <= len(tag) && tag[j:j+11] == "</location>" {
					locationEnd = j + 11
					break
				}
			}

			if locationEnd > 0 {
				locationCount++
				locationTag := tag[locationStart:locationEnd]
				file := extractXMLAttr(locationTag, "file")
				lineNum := extractXMLInt(locationTag, "line")
				column := extractXMLInt(locationTag, "column")

				// Use file0 as fallback if location doesn't have file
				if file == "" {
					file = file0
				}

				if file != "" && lineNum > 0 {
					results = append(results, AnalysisResult{
						Tool:     "Cppcheck",
						Severity: strings.ToLower(severity),
						File:     file,
						Line:     lineNum,
						Column:   column,
						Message:  msg,
						Rule:     id,
					})
				}
				i = locationEnd
			} else {
				i++
			}
		} else {
			i++
		}
	}

	// If no locations found but we have file0, create one result
	if len(results) == 0 && file0 != "" {
		// Try to extract line from error tag itself (fallback)
		lineNum := extractXMLInt(tag, "line")
		if lineNum > 0 {
			results = append(results, AnalysisResult{
				Tool:     "Cppcheck",
				Severity: strings.ToLower(severity),
				File:     file0,
				Line:     lineNum,
				Message:  msg,
				Rule:     id,
			})
		}
	}

	return results
}

func extractXMLAttr(line, attr string) string {
	prefix := attr + "=\""
	start := strings.Index(line, prefix)
	if start == -1 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(line[start:], "\"")
	if end == -1 {
		return ""
	}
	return line[start : start+end]
}

func extractXMLInt(line, attr string) int {
	val := extractXMLAttr(line, attr)
	if val == "" {
		return 0
	}
	var num int
	_, _ = fmt.Sscanf(val, "%d", &num)
	return num
}

func runLintAnalysis(vcpkg VcpkgSetup) ToolResults {
	result := ToolResults{
		Tool:    "clang-tidy",
		Status:  "success",
		Results: []AnalysisResult{},
	}

	// Check if clang-tidy is available
	if _, err := exec.LookPath("clang-tidy"); err != nil {
		result.Status = "skipped"
		result.Error = "clang-tidy not found"
		return result
	}

	// Set up vcpkg environment
	if err := vcpkg.SetupEnv(); err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("failed to setup vcpkg: %v", err)
		return result
	}

	// Verify compile_commands.json exists and get absolute path
	buildDir, err := filepath.Abs("build")
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("failed to get absolute path to build directory: %v", err)
		return result
	}

	compileDbPath := filepath.Join(buildDir, "compile_commands.json")
	if _, err := os.Stat(compileDbPath); os.IsNotExist(err) {
		result.Status = "skipped"
		result.Error = "compile_commands.json not found. Run 'cpx build' first."
		return result
	}

	// Find source files (same logic as LintCode)
	var files []string
	trackedFiles, err := GetGitTrackedCppFiles()
	if err != nil {
		// If not in git repo, fall back to scanning src/include directories
		for _, dir := range []string{".", "src", "include"} {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				// Skip build directories and other common ignored paths
				if strings.Contains(path, "/build/") || strings.Contains(path, "\\build\\") {
					return nil
				}
				ext := filepath.Ext(path)
				if ext == ".cpp" || ext == ".cc" || ext == ".cxx" || ext == ".c++" {
					files = append(files, path)
				}
				return nil
			})
		}
	} else {
		// Filter out files in build directories and other common ignored paths
		for _, file := range trackedFiles {
			// Skip files in build/, out/, bin/, .vcpkg/, etc.
			if strings.HasPrefix(file, "build/") ||
				strings.HasPrefix(file, "out/") ||
				strings.HasPrefix(file, "bin/") ||
				strings.HasPrefix(file, ".vcpkg/") ||
				strings.Contains(file, "/build/") ||
				strings.Contains(file, "\\build\\") {
				continue
			}
			files = append(files, file)
		}
	}

	if len(files) == 0 {
		result.Status = "skipped"
		result.Error = "no source files found"
		return result
	}

	// Get system include paths from the compiler to help clang-tidy find standard headers
	// This is needed because compile_commands.json might not have all system includes
	systemIncludes := GetSystemIncludePaths()

	// Run clang-tidy with absolute path to build directory
	tidyArgs := []string{"-p", buildDir}
	// Add system include paths as extra arguments
	for _, include := range systemIncludes {
		tidyArgs = append(tidyArgs, "--extra-arg=-isystem"+include)
	}
	tidyArgs = append(tidyArgs, files...)

	cmd := exec.Command("clang-tidy", tidyArgs...)
	output, _ := cmd.CombinedOutput()

	// Parse clang-tidy output
	results := parseClangTidyOutput(string(output))
	result.Results = results

	return result
}

func parseClangTidyOutput(output string) []AnalysisResult {
	results := []AnalysisResult{}

	lines := strings.Split(output, "\n")
	var currentFinding *AnalysisResult

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// clang-tidy format: file:line:column: severity: message [check-name]
		// Example: src/main.cpp:10:5: warning: unused variable 'x' [clang-diagnostic-unused-variable]
		// Notes follow warnings/errors: file:line:column: note: additional context
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			// If we have a current finding and this line doesn't match the format,
			// it might be a continuation of the message
			if currentFinding != nil && !strings.Contains(line, ":") {
				currentFinding.Message += " " + line
			}
			continue
		}

		file := parts[0]
		var lineNum, colNum int
		_, _ = fmt.Sscanf(parts[1], "%d", &lineNum)
		_, _ = fmt.Sscanf(parts[2], "%d", &colNum)

		severity := strings.TrimSpace(parts[3])
		message := ""
		rule := ""

		if len(parts) > 4 {
			message = strings.TrimSpace(parts[4])
			// Extract rule name from [rule-name]
			if idx := strings.Index(message, "["); idx != -1 {
				rule = message[idx+1:]
				if idx2 := strings.Index(rule, "]"); idx2 != -1 {
					rule = rule[:idx2]
				}
				message = strings.TrimSpace(message[:idx])
			}
		}

		severityLower := strings.ToLower(severity)

		// If this is a note, append it to the current finding
		if severityLower == "note" && currentFinding != nil {
			if message != "" {
				if currentFinding.Message != "" {
					currentFinding.Message += "; " + message
				} else {
					currentFinding.Message = message
				}
			}
			continue
		}

		// If this is a warning or error, create a new finding
		if (severityLower == "warning" || severityLower == "error") && file != "" && lineNum > 0 {
			// Save previous finding if exists
			if currentFinding != nil {
				results = append(results, *currentFinding)
			}

			// Create new finding
			currentFinding = &AnalysisResult{
				Tool:     "clang-tidy",
				Severity: severityLower,
				File:     file,
				Line:     lineNum,
				Column:   colNum,
				Message:  message,
				Rule:     rule,
			}
		} else {
			// Not a warning/error/note, reset current finding
			if currentFinding != nil {
				results = append(results, *currentFinding)
				currentFinding = nil
			}
		}
	}

	// Don't cpxt the last finding
	if currentFinding != nil {
		results = append(results, *currentFinding)
	}

	return results
}

func runFlawfinderAnalysis(targets []string) ToolResults {
	result := ToolResults{
		Tool:    "Flawfinder",
		Status:  "success",
		Results: []AnalysisResult{},
	}

	// Check if flawfinder is available
	if _, err := exec.LookPath("flawfinder"); err != nil {
		result.Status = "skipped"
		result.Error = "flawfinder not found"
		return result
	}

	// Discover source directories to scan (same as cppcheck)
	sourceDirs := discoverSourceDirectories(targets)
	if len(sourceDirs) == 0 {
		result.Status = "skipped"
		result.Error = "no source directories found to scan"
		return result
	}

	// Run flawfinder with CSV output
	// Pass directories to scan (flawfinder will scan all non-ignored files in those directories)
	args := []string{"--csv", "-m", "1"}
	args = append(args, sourceDirs...)

	cmd := exec.Command("flawfinder", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	// CSV output goes to stdout
	output := stdout.String()

	// Debug output if enabled
	if os.Getenv("CPX_DEBUG") != "" {
		fmt.Printf("Debug: flawfinder sourceDirs: %v\n", sourceDirs)
		fmt.Printf("Debug: flawfinder stdout length: %d\n", len(output))
		fmt.Printf("Debug: flawfinder stderr length: %d\n", stderr.Len())
		if len(output) > 0 {
			lines := strings.Split(output, "\n")
			fmt.Printf("Debug: flawfinder CSV lines: %d (first 3: %v)\n", len(lines), lines[:min(3, len(lines))])
		}
	}

	// Parse CSV output
	results := parseFlawfinderCSV(output)
	result.Results = results

	if os.Getenv("CPX_DEBUG") != "" {
		fmt.Printf("Debug: flawfinder parsed results: %d\n", len(results))
	}

	return result
}

func parseFlawfinderCSV(output string) []AnalysisResult {
	results := []AnalysisResult{}

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "File,") {
			continue
		}

		// Parse CSV line, handling quoted fields
		// Format: File,Line,Column,DefaultLevel,Level,Category,Name,Warning,Suggestion,Note,CWEs,Context,Fingerprint,ToolVersion,RuleId,HelpUri
		fields := parseCSVLine(line)
		if len(fields) < 7 {
			continue
		}

		file := fields[0]
		var lineNum, colNum, level int
		_, _ = fmt.Sscanf(fields[1], "%d", &lineNum)
		_, _ = fmt.Sscanf(fields[2], "%d", &colNum)
		_, _ = fmt.Sscanf(fields[4], "%d", &level) // Use Level (field 4), not DefaultLevel

		category := fields[5]
		name := fields[6]
		warning := ""
		if len(fields) > 7 {
			warning = fields[7]
		}

		// Use Warning as message, or combine with Suggestion if available
		message := warning
		if len(fields) > 8 && fields[8] != "" {
			if message != "" {
				message += ". " + fields[8] // Add suggestion
			} else {
				message = fields[8]
			}
		}

		// Convert level to severity
		severity := "info"
		if level >= 4 {
			severity = "error"
		} else if level >= 2 {
			severity = "warning"
		}

		if file != "" && lineNum > 0 {
			rule := name
			if category != "" {
				rule = fmt.Sprintf("%s: %s", category, name)
			}

			results = append(results, AnalysisResult{
				Tool:     "Flawfinder",
				Severity: severity,
				File:     file,
				Line:     lineNum,
				Column:   colNum,
				Message:  message,
				Rule:     rule,
			})
		}
	}

	return results
}

// parseCSVLine parses a CSV line, handling quoted fields that may contain commas
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(line); i++ {
		char := line[i]

		if char == '"' {
			// Check if it's an escaped quote ("")
			if i+1 < len(line) && line[i+1] == '"' {
				current.WriteByte('"')
				i++ // Skip next quote
			} else {
				// Toggle quote state
				inQuotes = !inQuotes
			}
		} else if char == ',' && !inQuotes {
			// Field separator
			fields = append(fields, current.String())
			current.Reset()
		} else {
			current.WriteByte(char)
		}
	}

	// Add last field
	fields = append(fields, current.String())

	return fields
}

func generateHTMLReport(analysis ComprehensiveAnalysis, outputFile string) error {
	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Cpx Code Analysis Report</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #0a0a1a 0%, #1a1a2e 50%, #16213e 100%);
            background-attachment: fixed;
            color: #e2e8f0;
            padding: 20px;
            line-height: 1.6;
            min-height: 100vh;
        }
        .container {
            max-width: 1600px;
            margin: 0 auto;
            background: rgba(15, 15, 35, 0.8);
            backdrop-filter: blur(20px);
            border: 1px solid rgba(0, 212, 255, 0.2);
            border-radius: 20px;
            padding: 40px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
        }
        .header {
            text-align: center;
            margin-bottom: 40px;
            padding-bottom: 30px;
            border-bottom: 2px solid rgba(0, 212, 255, 0.2);
        }
        h1 {
            background: linear-gradient(135deg, #00d4ff 0%, #00a8cc 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 10px;
            font-size: 3em;
            font-weight: 800;
            letter-spacing: -0.02em;
        }
        .timestamp {
            color: #94a3b8;
            font-size: 0.95em;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 8px;
        }
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }
        .summary-card {
            background: linear-gradient(135deg, rgba(0, 212, 255, 0.1) 0%, rgba(0, 168, 204, 0.05) 100%);
            border: 1px solid rgba(0, 212, 255, 0.2);
            border-radius: 16px;
            padding: 24px;
            transition: all 0.3s ease;
            position: relative;
            overflow: hidden;
        }
        .summary-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: 3px;
            background: linear-gradient(90deg, #00d4ff, #00a8cc);
        }
        .summary-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 24px rgba(0, 212, 255, 0.2);
            border-color: rgba(0, 212, 255, 0.4);
        }
        .summary-card h3 {
            color: #94a3b8;
            font-size: 0.85em;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 12px;
            display: flex;
            align-items: center;
            gap: 6px;
        }
        .summary-card .value {
            font-size: 2.5em;
            font-weight: 800;
            background: linear-gradient(135deg, #00d4ff 0%, #00a8cc 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }
        .tabs-container {
            margin-top: 40px;
        }
        .tabs {
            display: flex;
            gap: 12px;
            margin-bottom: 24px;
            border-bottom: 2px solid rgba(0, 212, 255, 0.2);
            overflow-x: auto;
        }
        .tab-button {
            padding: 14px 24px;
            background: rgba(0, 0, 0, 0.2);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-bottom: none;
            border-radius: 12px 12px 0 0;
            color: #94a3b8;
            font-size: 1em;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s ease;
            display: flex;
            align-items: center;
            gap: 10px;
            white-space: nowrap;
            position: relative;
            top: 2px;
        }
        .tab-button:hover {
            background: rgba(0, 212, 255, 0.1);
            color: #00d4ff;
            border-color: rgba(0, 212, 255, 0.3);
        }
        .tab-button.active {
            background: linear-gradient(135deg, rgba(0, 212, 255, 0.15) 0%, rgba(0, 168, 204, 0.1) 100%);
            color: #00d4ff;
            border-color: rgba(0, 212, 255, 0.3);
            border-bottom-color: transparent;
            top: 0;
        }
        .tab-content {
            display: none;
            background: rgba(0, 0, 0, 0.2);
            border-radius: 0 12px 12px 12px;
            padding: 24px;
            border: 1px solid rgba(255, 255, 255, 0.05);
            animation: fadeIn 0.3s ease;
        }
        .tab-content.active {
            display: block;
        }
        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: translateY(0); }
        }
        .tool-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 20px;
            background: linear-gradient(135deg, rgba(0, 212, 255, 0.1) 0%, rgba(0, 168, 204, 0.05) 100%);
            border: 1px solid rgba(0, 212, 255, 0.2);
            border-radius: 12px;
            margin-bottom: 20px;
        }
        .tool-header h2 {
            color: #00d4ff;
            font-size: 1.4em;
            font-weight: 700;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .tool-icon {
            font-size: 1.2em;
        }
        .tool-status {
            padding: 8px 18px;
            border-radius: 20px;
            font-size: 0.85em;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .status-success {
            background: linear-gradient(135deg, rgba(34, 197, 94, 0.2) 0%, rgba(22, 163, 74, 0.1) 100%);
            color: #22c55e;
            border: 1px solid rgba(34, 197, 94, 0.3);
        }
        .status-error {
            background: linear-gradient(135deg, rgba(239, 68, 68, 0.2) 0%, rgba(220, 38, 38, 0.1) 100%);
            color: #ef4444;
            border: 1px solid rgba(239, 68, 68, 0.3);
        }
        .status-skipped {
            background: rgba(148, 163, 184, 0.15);
            color: #94a3b8;
            border: 1px solid rgba(148, 163, 184, 0.2);
        }
        .findings-table {
            width: 100%;
            border-collapse: separate;
            border-spacing: 0;
            background: rgba(0, 0, 0, 0.3);
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        }
        .findings-table th {
            background: linear-gradient(135deg, rgba(0, 212, 255, 0.15) 0%, rgba(0, 168, 204, 0.1) 100%);
            padding: 16px;
            text-align: left;
            color: #00d4ff;
            font-weight: 700;
            font-size: 0.9em;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            border-bottom: 2px solid rgba(0, 212, 255, 0.3);
        }
        .findings-table th:first-child { border-top-left-radius: 12px; }
        .findings-table th:last-child { border-top-right-radius: 12px; }
        .findings-table td {
            padding: 16px;
            border-bottom: 1px solid rgba(255, 255, 255, 0.05);
        }
        .findings-table tbody tr {
            transition: all 0.2s ease;
        }
        .findings-table tbody tr:nth-child(even) {
            background: rgba(255, 255, 255, 0.02);
        }
        .findings-table tbody tr:hover {
            background: rgba(0, 212, 255, 0.1);
            transform: scale(1.01);
        }
        .findings-table tbody tr:last-child td:first-child {
            border-bottom-left-radius: 12px;
        }
        .findings-table tbody tr:last-child td:last-child {
            border-bottom-right-radius: 12px;
        }
        .severity {
            padding: 6px 14px;
            border-radius: 8px;
            font-size: 0.8em;
            font-weight: 700;
            display: inline-flex;
            align-items: center;
            gap: 6px;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .severity-error {
            background: linear-gradient(135deg, rgba(239, 68, 68, 0.25) 0%, rgba(220, 38, 38, 0.15) 100%);
            color: #ff6b6b;
            border: 1px solid rgba(239, 68, 68, 0.4);
        }
        .severity-warning {
            background: linear-gradient(135deg, rgba(251, 191, 36, 0.25) 0%, rgba(245, 158, 11, 0.15) 100%);
            color: #fbbf24;
            border: 1px solid rgba(251, 191, 36, 0.4);
        }
        .severity-info {
            background: linear-gradient(135deg, rgba(59, 130, 246, 0.25) 0%, rgba(37, 99, 235, 0.15) 100%);
            color: #60a5fa;
            border: 1px solid rgba(59, 130, 246, 0.4);
        }
        .file-path {
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Courier New', monospace;
            color: #cbd5e1;
            font-size: 0.9em;
            background: rgba(0, 0, 0, 0.3);
            padding: 4px 8px;
            border-radius: 6px;
            display: inline-block;
        }
        .line-number {
            color: #00d4ff;
            font-weight: 700;
            font-size: 1.1em;
            background: rgba(0, 212, 255, 0.15);
            padding: 4px 10px;
            border-radius: 6px;
            display: inline-block;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        }
        .message {
            color: #e2e8f0;
            line-height: 1.5;
            max-width: 500px;
        }
        .rule {
            color: #94a3b8;
            font-size: 0.85em;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            background: rgba(148, 163, 184, 0.1);
            padding: 4px 8px;
            border-radius: 6px;
            display: inline-block;
        }
        .no-findings {
            text-align: center;
            padding: 60px 40px;
            color: #22c55e;
            font-size: 1.3em;
            background: rgba(34, 197, 94, 0.1);
            border: 2px dashed rgba(34, 197, 94, 0.3);
            border-radius: 12px;
        }
        .error-message {
            background: linear-gradient(135deg, rgba(239, 68, 68, 0.15) 0%, rgba(220, 38, 38, 0.1) 100%);
            border: 1px solid rgba(239, 68, 68, 0.4);
            border-radius: 12px;
            padding: 20px;
            color: #ff6b6b;
            margin-top: 10px;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .severity-icon {
            font-size: 1.1em;
        }
        @media (max-width: 768px) {
            .container {
                padding: 20px;
            }
            h1 {
                font-size: 2em;
            }
            .summary {
                grid-template-columns: 1fr;
            }
            .tabs {
                flex-direction: column;
                border-bottom: none;
            }
            .tab-button {
                border-radius: 12px;
                border: 1px solid rgba(255, 255, 255, 0.1);
                top: 0;
                margin-bottom: 8px;
            }
            .tab-button.active {
                border-color: rgba(0, 212, 255, 0.3);
            }
            .tab-content {
                border-radius: 12px;
            }
            .findings-table {
                font-size: 0.85em;
            }
            .findings-table th,
            .findings-table td {
                padding: 10px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Cpx Code Analysis Report</h1>
            <div class="timestamp">
                <span></span>
                <span>Generated: {{.Timestamp.Format "2006-01-02 15:04:05"}}</span>
            </div>
        </div>

        <div class="summary">
            <div class="summary-card">
                <h3>Total Findings</h3>
                <div class="value">{{.Summary.TotalFindings}}</div>
            </div>
            {{range $severity, $count := .Summary.BySeverity}}
            <div class="summary-card">
                <h3>
                    {{if eq $severity "error"}}{{else if eq $severity "warning"}}{{else}}{{end}}
                    {{$severity}}
                </h3>
                <div class="value">{{$count}}</div>
            </div>
            {{end}}
        </div>

        <div class="tabs-container">
            <div class="tabs">
                {{range $index, $tool := .Tools}}
                <button class="tab-button {{if eq $index 0}}active{{end}}" onclick="showTab({{$index}})">
                    <span>
                        {{if eq $tool.Tool "Cppcheck"}}{{else if eq $tool.Tool "clang-tidy"}}{{else if eq $tool.Tool "Flawfinder"}}{{else}}{{end}}
                    </span>
                    <span>{{$tool.Tool}}</span>
                    {{if gt (len $tool.Results) 0}}
                    <span style="font-size: 0.85em; opacity: 0.7;">({{len $tool.Results}})</span>
                    {{end}}
                </button>
                {{end}}
            </div>

            {{range $index, $tool := .Tools}}
            <div class="tab-content {{if eq $index 0}}active{{end}}" id="tab-{{$index}}">
                <div class="tool-header">
                    <h2>
                        <span class="tool-icon">
                            {{if eq $tool.Tool "Cppcheck"}}{{else if eq $tool.Tool "clang-tidy"}}{{else if eq $tool.Tool "Flawfinder"}}{{else}}{{end}}
                        </span>
                        {{$tool.Tool}}
                        {{if gt (len $tool.Results) 0}}
                        <span style="font-size: 0.7em; color: #94a3b8; font-weight: 400;">({{len $tool.Results}} findings)</span>
                        {{end}}
                    </h2>
                    <span class="tool-status status-{{$tool.Status}}">{{$tool.Status}}</span>
                </div>
                {{if $tool.Error}}
                <div class="error-message">
                    <span></span>
                    <span>Error: {{$tool.Error}}</span>
                </div>
                {{else if eq (len $tool.Results) 0}}
                <div class="no-findings">No issues found!</div>
                {{else}}
                <table class="findings-table">
                    <thead>
                        <tr>
                            <th>Severity</th>
                            <th>File</th>
                            <th>Line</th>
                            <th>Message</th>
                            <th>Rule</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $tool.Results}}
                        <tr>
                            <td>
                                <span class="severity severity-{{.Severity}}">
                                    <span class="severity-icon">
                                        {{if eq .Severity "error"}}{{else if eq .Severity "warning"}}{{else}}{{end}}
                                    </span>
                                    {{.Severity}}
                                </span>
                            </td>
                            <td><span class="file-path">{{.File}}</span></td>
                            <td><span class="line-number">{{.Line}}</span></td>
                            <td><span class="message">{{.Message}}</span></td>
                            <td><span class="rule">{{.Rule}}</span></td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
                {{end}}
            </div>
            {{end}}
        </div>
    </div>

    <script>
        function showTab(index) {
            // Hide all tabs
            const tabs = document.querySelectorAll('.tab-content');
            const buttons = document.querySelectorAll('.tab-button');

            tabs.forEach(tab => tab.classList.remove('active'));
            buttons.forEach(btn => btn.classList.remove('active'));

            // Show selected tab
            document.getElementById('tab-' + index).classList.add('active');
            buttons[index].classList.add('active');
        }
    </script>
</body>
</html>`

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, analysis); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
