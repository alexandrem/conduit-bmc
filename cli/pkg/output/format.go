package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Format represents the output format type
type Format string

const (
	// FormatText is the default human-readable text format
	FormatText Format = "text"
	// FormatJSON is the JSON output format
	FormatJSON Format = "json"
)

// Formatter handles different output formats
type Formatter struct {
	format Format
	writer io.Writer
}

// New creates a new Formatter with the specified format
func New(format Format) *Formatter {
	return &Formatter{
		format: format,
		writer: os.Stdout,
	}
}

// SetWriter sets a custom writer for output (useful for testing)
func (f *Formatter) SetWriter(w io.Writer) {
	f.writer = w
}

// Output writes the data in the configured format
func (f *Formatter) Output(data interface{}) error {
	switch f.format {
	case FormatJSON:
		return f.outputJSON(data)
	case FormatText:
		// For text format, we expect the caller to handle formatting
		// This is just a fallback
		fmt.Fprintf(f.writer, "%v\n", data)
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", f.format)
	}
}

// outputJSON marshals and outputs data as JSON
func (f *Formatter) outputJSON(data interface{}) error {
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// IsJSON returns true if the format is JSON
func (f *Formatter) IsJSON() bool {
	return f.format == FormatJSON
}

// IsText returns true if the format is text
func (f *Formatter) IsText() bool {
	return f.format == FormatText
}

// AddFormatFlag adds a --output flag to a cobra command
// This should be called in the init() function for commands that support output formatting
func AddFormatFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("output", "o", "text", "Output format (text|json)")
}

// GetFormatFromCmd extracts the output format from a cobra command's flags
func GetFormatFromCmd(cmd *cobra.Command) (Format, error) {
	formatStr, err := cmd.Flags().GetString("output")
	if err != nil {
		return FormatText, err
	}

	format := Format(formatStr)
	switch format {
	case FormatText, FormatJSON:
		return format, nil
	default:
		return FormatText, fmt.Errorf("invalid output format: %s (must be 'text' or 'json')", formatStr)
	}
}
