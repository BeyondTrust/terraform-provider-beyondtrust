package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "codegen",
	Short:   "Terraform provider code generator for BeyondTrust SMOP",
	Version: version,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from OpenAPI spec",
	RunE:  runGenerate,
}

var (
	specPath     string
	outputPath   string
	resourceName string
)

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVar(&specPath, "spec", "", "Path to OpenAPI specification (required)")
	generateCmd.Flags().StringVar(&resourceName, "resource", "", "Resource to generate")
	generateCmd.Flags().StringVar(&outputPath, "output", "", "Output file path")
	generateCmd.MarkFlagRequired("spec")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Generating from OpenAPI spec: %s\n", specPath)
	fmt.Printf("Resource: %s\n", resourceName)
	fmt.Printf("Output: %s\n", outputPath)
	return nil
}
