package checker

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/oasdiff/diff"
	"github.com/oasdiff/oasdiff/load"
	"github.com/stretchr/testify/require"
)

func loadOpenAPISpecInfo(t *testing.T, path string) *load.SpecInfo {
	loader := &openapi3.Loader{IsExternalRefsAllowed: true}
	source := load.NewSource(path)
	specInfo, err := load.NewSpecInfo(loader, source)
	require.NoError(t, err)
	return specInfo
}

func TestPropertyDeprecationCheck(t *testing.T) {
	// Setup config with deprecation days and log levels for all property checks
	config := NewConfig(nil).
		WithDeprecation(30, 30)
	config.LogLevels[PropertyDeprecatedSunsetMissingId] = INFO
	config.LogLevels[PropertyDeprecatedSunsetParseId] = INFO
	config.LogLevels[PropertyDeprecatedId] = INFO

	// Mock schema diff with deprecated property
	schemaDiff := &diff.SchemaDiff{
		PropertiesDiff: &diff.SchemasDiff{
			Modified: map[string]*diff.SchemaDiff{
				"deprecatedProp": {
					DeprecatedDiff: &diff.ValueDiff{To: true},
					Revision: &openapi3.Schema{
						Extensions: map[string]interface{}{
							"x-stability-level": "stable",
							"x-sunset":          "2026-12-31",
						},
					},
				},
			},
		},
	}

	result := make(Changes, 0)
	reportedProperties := make(map[string]bool)
	revision := &openapi3.Operation{}

	PropertyDeprecationCheck(schemaDiff, "", &result, config, nil, revision, "GET", "/test", reportedProperties)

	if len(result) != 1 {
		t.Errorf("Expected 1 property change, got %d", len(result))
	}

	foundDeprecated := false
	for _, c := range result {
		if c.GetId() == PropertyDeprecatedId {
			foundDeprecated = true
		}
	}
	if !foundDeprecated {
		t.Error("Property deprecation change not found")
	}
}

func TestPropertyDeprecationCheck_SunsetMissing(t *testing.T) {
	config := NewConfig(nil).
		WithDeprecation(30, 30)
	config.LogLevels[PropertyDeprecatedSunsetMissingId] = INFO

	schemaDiff := &diff.SchemaDiff{
		PropertiesDiff: &diff.SchemasDiff{
			Modified: map[string]*diff.SchemaDiff{
				"deprecatedProp": {
					DeprecatedDiff: &diff.ValueDiff{To: true},
					Revision: &openapi3.Schema{
						Extensions: map[string]interface{}{
							"x-stability-level": "stable",
							// No x-sunset
						},
					},
				},
			},
		},
	}

	result := make(Changes, 0)
	reportedProperties := make(map[string]bool)
	revision := &openapi3.Operation{}

	PropertyDeprecationCheck(schemaDiff, "", &result, config, nil, revision, "GET", "/test", reportedProperties)

	if len(result) != 1 {
		t.Errorf("Expected 1 property change, got %d", len(result))
	}

	found := false
	for _, c := range result {
		if c.GetId() == PropertyDeprecatedSunsetMissingId {
			found = true
		}
	}
	if !found {
		t.Error("PropertyDeprecatedSunsetMissingId change not found")
	}
}

func TestPropertyDeprecationCheck_AllOf(t *testing.T) {
	config := NewConfig(nil).
		WithDeprecation(30, 30)
	config.LogLevels[PropertyDeprecatedId] = INFO

	schemaDiff := &diff.SchemaDiff{
		AllOfDiff: &diff.SubschemasDiff{
			Modified: diff.ModifiedSubschemas{
				{
					Diff: &diff.SchemaDiff{
						PropertiesDiff: &diff.SchemasDiff{
							Modified: map[string]*diff.SchemaDiff{
								"allOfProp": {
									DeprecatedDiff: &diff.ValueDiff{To: true},
									Revision: &openapi3.Schema{
										Extensions: map[string]interface{}{
											"x-stability-level": "stable",
											"x-sunset":          "2026-12-31",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := make(Changes, 0)
	reportedProperties := make(map[string]bool)
	revision := &openapi3.Operation{}

	PropertyDeprecationCheck(schemaDiff, "", &result, config, nil, revision, "GET", "/test", reportedProperties)

	if len(result) != 1 {
		t.Errorf("Expected 1 property change, got %d", len(result))
	}

	found := false
	for _, c := range result {
		if c.GetId() == PropertyDeprecatedId {
			found = true
			t.Logf("Found deprecated allOf property: %+v", c)
		}
	}
	if !found {
		t.Error("PropertyDeprecatedId change not found in allOf")
	}
}

// getMapKeys returns the keys of a map as a slice
func getMapKeys(m interface{}) []string {
	keys := []string{}
	switch mm := m.(type) {
	case map[string]*diff.SchemaDiff:
		for k := range mm {
			keys = append(keys, k)
		}
	}
	return keys
}
