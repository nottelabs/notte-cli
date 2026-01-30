package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	specFile := flag.String("spec", "", "Path to OpenAPI spec file (required)")
	outputDir := flag.String("output", "", "Output directory for generated files (required)")
	flag.Parse()

	if *specFile == "" || *outputDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: gen-flags -spec <openapi.json> -output <output-dir>\n")
		os.Exit(1)
	}

	if err := run(*specFile, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(specFile, outputDir string) error {
	fmt.Printf("Parsing OpenAPI spec from %s...\n", specFile)

	// Parse OpenAPI spec
	spec, err := ParseOpenAPISpec(specFile)
	if err != nil {
		return fmt.Errorf("failed to parse spec: %w", err)
	}

	// Build schema map
	schemas := buildSchemaMap(spec)

	// Extract command configurations
	configs, err := ExtractCommandConfigs(spec)
	if err != nil {
		return fmt.Errorf("failed to extract commands: %w", err)
	}

	fmt.Printf("Found %d commands to generate\n", len(configs))

	// Track all errors
	var allErrors []GenerationError
	var generatedFiles []string
	var partialFiles []string

	// Generate flags for each command
	for _, config := range configs {
		fmt.Printf("  Generating flags for %s...\n", config.Name)

		code, errors, err := GenerateFlagsFile(config, schemas)
		if err != nil {
			return fmt.Errorf("failed to generate %s: %w", config.Name, err)
		}

		// Collect errors but continue if there's partial code
		if len(errors) > 0 {
			allErrors = append(allErrors, errors...)

			// Count supported fields
			supportedFields := 0
			for _, fc := range config.Fields {
				if fc.Category != CategoryUnsupported {
					supportedFields++
				}
			}

			// If NO fields are supported, skip this command entirely
			if supportedFields == 0 {
				fmt.Printf("    ⚠ Skipped %s (no supported fields)\n", config.Name)
				continue
			}

			// Write partial file with warning comments
			partialFiles = append(partialFiles, config.Name)
		}

		// Write output file
		filename := fmt.Sprintf("%s_flags.gen.go", toSnakeCase(strings.ToLower(config.Name)))
		outputPath := filepath.Join(outputDir, filename)

		if err := os.WriteFile(outputPath, []byte(code), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputPath, err)
		}

		generatedFiles = append(generatedFiles, filename)
		if len(errors) > 0 {
			fmt.Printf("    ⚠ Generated %s (partial - %d unsupported fields)\n", filename, len(errors))
		} else {
			fmt.Printf("    ✓ Generated %s\n", filename)
		}
	}

	// Report errors (as warnings since we generated partial files)
	if len(allErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\n════════════════════════════════════════════════════════════════\n")
		fmt.Fprintf(os.Stderr, "⚠ FLAG GENERATION COMPLETED WITH WARNINGS\n")
		fmt.Fprintf(os.Stderr, "════════════════════════════════════════════════════════════════\n")
		fmt.Fprintf(os.Stderr, "\nFound %d unsupported field(s) that require manual handling:\n", len(allErrors))

		for i, err := range allErrors {
			fmt.Fprintf(os.Stderr, "\n─────────────────────────── Warning %d/%d ───────────────────────────\n", i+1, len(allErrors))
			fmt.Fprintf(os.Stderr, "%s", err.Error())
		}

		fmt.Fprintf(os.Stderr, "\n════════════════════════════════════════════════════════════════\n")
		if len(partialFiles) > 0 {
			fmt.Fprintf(os.Stderr, "\nPartial files generated for: %v\n", partialFiles)
			fmt.Fprintf(os.Stderr, "These files contain flags for supported fields only.\n")
			fmt.Fprintf(os.Stderr, "You must manually handle the unsupported fields listed above.\n")
		}
		fmt.Fprintf(os.Stderr, "\nSee docs/flag-generation.md for handling complex types.\n")
		fmt.Fprintf(os.Stderr, "════════════════════════════════════════════════════════════════\n\n")
	}

	// Success summary
	fmt.Printf("\n✓ Successfully generated %d flag files\n", len(generatedFiles))
	fmt.Printf("\nGenerated files:\n")
	for _, f := range generatedFiles {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Review generated files in %s\n", outputDir)
	fmt.Printf("  2. Update command files to use Register*Flags() and Build*Request()\n")
	fmt.Printf("  3. Remove manual flag declarations\n")
	fmt.Printf("  4. Test the commands\n\n")

	return nil
}
