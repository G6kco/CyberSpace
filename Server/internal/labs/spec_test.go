package labs

import (
	"strings"
	"testing"
)

func TestParseSpec(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		image   string
		wantErr string
	}{
		{name: "image only", yaml: "image: scenario1", image: "scenario1"},
		{name: "image is trimmed", yaml: "image: '  scenario1:latest '", image: "scenario1:latest"},
		{
			name:  "other keys ignored",
			yaml:  "version: \"1\"\nimage: scenario7\nnetwork:\n  mode: macvlan\n",
			image: "scenario7",
		},
		{name: "upper-case tag allowed", yaml: "image: scenario1:V2", image: "scenario1:V2"},
		{name: "registry port allowed", yaml: "image: registry:5000/scenario1", image: "registry:5000/scenario1"},
		{name: "missing image", yaml: "version: \"1\"", wantErr: "no image"},
		{name: "blank image", yaml: "image: '  '", wantErr: "no image"},
		{name: "upper-case repository", yaml: "image: Scenario1", wantErr: "lower case"},
		{name: "invalid yaml", yaml: "image: [", wantErr: "parse lab spec"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseSpec(tt.yaml)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseSpec() error = %v; want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSpec() error = %v", err)
			}
			if spec.Image != tt.image {
				t.Fatalf("Image = %q; want %q", spec.Image, tt.image)
			}
		})
	}
}
