package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFormatter_OutputJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name: "simple struct",
			data: struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{
				Name:  "test",
				Value: 42,
			},
			wantErr: false,
		},
		{
			name: "map",
			data: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:    "slice",
			data:    []string{"item1", "item2", "item3"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := New(FormatJSON)
			formatter.SetWriter(&buf)

			err := formatter.Output(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Output() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				output := buf.String()
				if output == "" {
					t.Error("Output() produced empty output")
				}
				// Verify it's valid JSON by checking for basic structure
				if !strings.Contains(output, "{") && !strings.Contains(output, "[") {
					t.Errorf("Output() doesn't appear to be JSON: %s", output)
				}
			}
		})
	}
}

func TestFormatter_IsJSON(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		want   bool
	}{
		{
			name:   "json format",
			format: FormatJSON,
			want:   true,
		},
		{
			name:   "text format",
			format: FormatText,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.format)
			if got := f.IsJSON(); got != tt.want {
				t.Errorf("IsJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFormatFromCmd(t *testing.T) {
	tests := []struct {
		name       string
		flagValue  string
		want       Format
		wantErr    bool
		errMessage string
	}{
		{
			name:      "json format",
			flagValue: "json",
			want:      FormatJSON,
			wantErr:   false,
		},
		{
			name:      "text format",
			flagValue: "text",
			want:      FormatText,
			wantErr:   false,
		},
		{
			name:       "invalid format",
			flagValue:  "xml",
			want:       FormatText,
			wantErr:    true,
			errMessage: "invalid output format: xml",
		},
		{
			name:       "empty format",
			flagValue:  "",
			want:       FormatText,
			wantErr:    true,
			errMessage: "invalid output format:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use: "test",
			}
			AddFormatFlag(cmd)
			cmd.Flags().Set("output", tt.flagValue)

			got, err := GetFormatFromCmd(cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFormatFromCmd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMessage) {
				t.Errorf("GetFormatFromCmd() error = %v, want error containing %q", err, tt.errMessage)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetFormatFromCmd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddFormatFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	AddFormatFlag(cmd)

	flag := cmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("AddFormatFlag() did not add 'output' flag")
	}

	if flag.Shorthand != "o" {
		t.Errorf("AddFormatFlag() shorthand = %q, want %q", flag.Shorthand, "o")
	}

	if flag.DefValue != "text" {
		t.Errorf("AddFormatFlag() default value = %q, want %q", flag.DefValue, "text")
	}
}
