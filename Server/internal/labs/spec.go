package labs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// Spec is the part of a lab_template_versions.yaml_spec the engine uses.
// Other keys are allowed and ignored, so the documented YAML format can grow
// without breaking provisioning.
type Spec struct {
	Version string `yaml:"version"`
	Image   string `yaml:"image"`
}

// ParseSpec reads and validates a lab template's YAML.
func ParseSpec(raw string) (Spec, error) {
	var spec Spec
	if err := yaml.Unmarshal([]byte(raw), &spec); err != nil {
		return Spec{}, fmt.Errorf("parse lab spec: %w", err)
	}

	spec.Image = strings.TrimSpace(spec.Image)
	if spec.Image == "" {
		return Spec{}, errors.New("lab spec has no image")
	}
	// Docker rejects upper-case repository names (tags may use either case);
	// the reference service lowered them silently, but a wrong spec is better
	// caught loudly.
	if repository := imageRepository(spec.Image); repository != strings.ToLower(repository) {
		return Spec{}, fmt.Errorf("lab spec image %q: repository name must be lower case", spec.Image)
	}
	return spec, nil
}

// imageRepository strips the tag and digest from an image reference. The tag
// separator is the last ':' after the last '/', since a registry host may
// carry a port ("registry:5000/scenario1:latest").
func imageRepository(image string) string {
	if at := strings.IndexByte(image, '@'); at >= 0 {
		image = image[:at]
	}
	if colon := strings.LastIndexByte(image, ':'); colon > strings.LastIndexByte(image, '/') {
		image = image[:colon]
	}
	return image
}
