package diagnostics

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/open-southeners/php-lsp/internal/config"
	"github.com/open-southeners/php-lsp/internal/protocol"
)

type phpstanRunner struct {
	binPath  string
	level    string
	cfgFile  string
	rootPath string
	logger   *log.Logger
	enabled  bool
}

type phpstanOutput struct {
	Totals struct {
		Errors     int `json:"errors"`
		FileErrors int `json:"file_errors"`
	} `json:"totals"`
	Files map[string]phpstanFileResult `json:"files"`
}

type phpstanFileResult struct {
	Errors   int              `json:"errors"`
	Messages []phpstanMessage `json:"messages"`
}

type phpstanMessage struct {
	Message    string `json:"message"`
	Line       int    `json:"line"`
	Ignorable  bool   `json:"ignorable"`
	Identifier string `json:"identifier"`
}

func newPHPStanRunner(rootPath string, cfg *config.Config, logger *log.Logger) *phpstanRunner {
	r := &phpstanRunner{
		rootPath: rootPath,
		logger:   logger,
		level:    cfg.PHPStanLevel,
	}

	if cfg.PHPStanEnabled != nil {
		r.enabled = *cfg.PHPStanEnabled
	} else {
		r.enabled = true
	}

	if cfg.PHPStanPath != "" {
		r.binPath = cfg.PHPStanPath
	} else {
		vendorBin := filepath.Join(rootPath, "vendor", "bin", "phpstan")
		if _, err := os.Stat(vendorBin); err == nil {
			r.binPath = vendorBin
		} else if path, err := exec.LookPath("phpstan"); err == nil {
			r.binPath = path
		} else {
			r.enabled = false
		}
	}

	if cfg.PHPStanConfig != "" {
		r.cfgFile = cfg.PHPStanConfig
	} else {
		for _, name := range []string{"phpstan.neon", "phpstan.neon.dist", "phpstan.dist.neon"} {
			candidate := filepath.Join(rootPath, name)
			if _, err := os.Stat(candidate); err == nil {
				r.cfgFile = candidate
				break
			}
		}
	}

	if r.enabled && r.binPath != "" {
		logger.Printf("PHPStan: binary=%s config=%s level=%s", r.binPath, r.cfgFile, r.level)
	}

	return r
}

func (r *phpstanRunner) available() bool {
	return r.enabled && r.binPath != ""
}

func (r *phpstanRunner) analyze(filePath string) []protocol.Diagnostic {
	args := []string{"analyse", "--error-format=json", "--no-progress", "--no-ansi"}

	if r.cfgFile != "" {
		args = append(args, "-c", r.cfgFile)
	}

	if r.level != "" {
		args = append(args, "--level="+r.level)
	}

	args = append(args, filePath)

	cmd := exec.Command(r.binPath, args...)
	cmd.Dir = r.rootPath

	// Output() returns stdout; on non-zero exit, err is *ExitError
	// but stdout still contains the JSON output.
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				// Exit code 1 = errors found (expected), anything else is a real error
				r.logger.Printf("PHPStan error (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
				return nil
			}
			// Exit code 1: errors found, stdout has JSON — fall through to parse
		} else {
			r.logger.Printf("PHPStan exec error: %v", err)
			return nil
		}
	}

	return r.parseOutput(output)
}

func (r *phpstanRunner) parseOutput(data []byte) []protocol.Diagnostic {
	jsonData := extractJSON(data)
	if jsonData == nil {
		return nil
	}

	var result phpstanOutput
	if err := json.Unmarshal(jsonData, &result); err != nil {
		r.logger.Printf("PHPStan JSON parse error: %v", err)
		return nil
	}

	var diags []protocol.Diagnostic
	for _, fileResult := range result.Files {
		for _, msg := range fileResult.Messages {
			line := msg.Line - 1 // PHPStan 1-based → LSP 0-based
			if line < 0 {
				line = 0
			}

			code := msg.Identifier
			if code == "" {
				code = "phpstan"
			}

			diags = append(diags, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: line, Character: 0},
					End:   protocol.Position{Line: line, Character: 0},
				},
				Severity: protocol.DiagnosticSeverityError,
				Source:   "phpstan",
				Message:  msg.Message,
				Code:     code,
			})
		}
	}

	return diags
}

// extractJSON finds the first JSON object in data, skipping any preamble.
func extractJSON(data []byte) []byte {
	s := strings.TrimSpace(string(data))
	start := strings.Index(s, "{")
	if start < 0 {
		return nil
	}
	return []byte(s[start:])
}
