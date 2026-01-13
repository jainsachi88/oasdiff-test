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
	config.LogLevels[PropertyReactivatedId] = INFO
	config.LogLevels[PropertyDeprecatedSunsetMissingId] = INFO
	config.LogLevels[PropertySunsetDateTooSmallId] = INFO
	config.LogLevels[PropertyDeprecatedId] = INFO

	// Mock diff with both deprecated and reactivated properties
	d := &diff.Diff{
		ComponentsDiff: &diff.ComponentsDiff{
			SchemasDiff: &diff.SchemasDiff{
				Modified: map[string]*diff.SchemaDiff{
					"TestSchema": {
						PropertiesDiff: &diff.SchemasDiff{
							Modified: map[string]*diff.SchemaDiff{
								"deprecatedProp": {
									DeprecatedDiff: &diff.ValueDiff{To: true},
									Revision: &openapi3.Schema{
										Extensions: map[string]interface{}{
											"x-stability-level": "stable",
											"x-sunset":          "2026-12-31", // far future sunset date
										},
									},
								},
								"reactivatedProp": {
									DeprecatedDiff: &diff.ValueDiff{To: false},
									Revision: &openapi3.Schema{
										Extensions: map[string]interface{}{
											"x-stability-level": "stable",
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

	changes := PropertyDeprecationCheck(d, nil, config)

	if len(changes) != 2 {
		t.Errorf("Expected 2 property changes, got %d", len(changes))
	}

	foundDeprecated := false
	foundReactivated := false
	for _, c := range changes {
		if c.GetId() == PropertyDeprecatedId {
			foundDeprecated = true
		}
		if c.GetId() == PropertyReactivatedId {
			foundReactivated = true
		}
	}
	if !foundDeprecated {
		t.Error("Property deprecation change not found")
	}
	if !foundReactivated {
		t.Error("Property reactivation change not found")
	}
}

func TestPropertyDeprecationCheck_WithSampleFiles(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "testdata/property_deprecation_base.yaml")
	spec := loadOpenAPISpecInfo(t, "testdata/property_deprecation_spec.yaml")

	// Debug print: show loaded spec titles and versions
	if base.Spec != nil && base.Spec.Info != nil {
		t.Logf("Base spec title: %s, version: %s", base.Spec.Info.Title, base.Spec.Info.Version)
	}
	if spec.Spec != nil && spec.Spec.Info != nil {
		t.Logf("Spec spec title: %s, version: %s", spec.Spec.Info.Title, spec.Spec.Info.Version)
	}

	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)

	// Debug print: show what schemas are marked as modified
	if d.ComponentsDiff != nil && d.ComponentsDiff.SchemasDiff != nil {
		t.Logf("SchemasDiff.Modified keys: %v", getMapKeys(d.ComponentsDiff.SchemasDiff.Modified))
		for schema, schemaDiff := range d.ComponentsDiff.SchemasDiff.Modified {
			if schemaDiff.PropertiesDiff != nil {
				t.Logf("Schema '%s' PropertiesDiff.Modified keys: %v", schema, getMapKeys(schemaDiff.PropertiesDiff.Modified))
			}
		}
	}

	config := NewConfig(nil)
	changes := PropertyDeprecationCheck(d, osm, config)

	found := false
	for _, c := range changes {
		if c.GetId() == PropertyDeprecatedId {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected PropertyDeprecatedId in changes, got: %+v", changes)
	}
}

func TestPropertyDeprecationCheck_NestedProperty(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "/testdata/property_deprecation_base.yaml")
	spec := loadOpenAPISpecInfo(t, "/testdata/property_deprecation_spec.yaml")

	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)

	config := NewConfig(nil)
	changes := PropertyDeprecationCheck(d, osm, config)

	found := false
	for _, c := range changes {
		if c.GetId() == PropertyDeprecatedId && c.GetOperation() == "TestSchema" && c.GetPath() == "parent.nestedProp" {
			found = true
			t.Logf("Found deprecated nested property: TestSchema.parent.nestedProp")
		}
	}
	if !found {
		t.Errorf("Expected PropertyDeprecatedId for TestSchema.parent.nestedProp in changes, got: %+v", changes)
	}
}

func TestPropertyDeprecationCheck_AllOfProperty(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "testdata/property_deprecation_allof_base.yaml")
	spec := loadOpenAPISpecInfo(t, "testdata/property_deprecation_allof_spec.yaml")
	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)

	// Debug print: show what schemas are marked as modified
	t.Logf("SchemasDiff.Modified keys: %v", getMapKeys(d.ComponentsDiff.SchemasDiff.Modified))
	if d.ComponentsDiff.SchemasDiff.Modified["AllOfSchema"] != nil {
		t.Logf("AllOfSchema diff: %+v", d.ComponentsDiff.SchemasDiff.Modified["AllOfSchema"])
	}

	config := NewConfig(nil)
	changes := PropertyDeprecationCheck(d, osm, config)
	found := false
	for _, c := range changes {
		if c.GetId() == PropertyDeprecatedId && c.GetOperation() == "AllOfSchema" && c.GetPath() == "allOfProp" {
			found = true
			t.Logf("Found deprecated allOf property: AllOfSchema.allOfProp")
		}
	}
	if !found {
		t.Errorf("Expected PropertyDeprecatedId for AllOfSchema.allOfProp in changes, got: %+v", changes)
	}
}

func TestPropertyDeprecationCheck_OneOfProperty(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "checker/testdata/property_deprecation_oneof_base.yaml")
	spec := loadOpenAPISpecInfo(t, "checker/testdata/property_deprecation_oneof_spec.yaml")
	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)
	config := NewConfig(nil)
	changes := PropertyDeprecationCheck(d, osm, config)
	found := false
	for _, c := range changes {
		if c.GetId() == PropertyDeprecatedId && c.GetOperation() == "OneOfSchema" && c.GetPath() == "oneOfProp" {
			found = true
			t.Logf("Found deprecated oneOf property: OneOfSchema.oneOfProp")
		}
	}
	if !found {
		t.Errorf("Expected PropertyDeprecatedId for OneOfSchema.oneOfProp in changes, got: %+v", changes)
	}
}

func TestPropertyDeprecationCheck_AnyOfProperty(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "checker/testdata/property_deprecation_anyof_base.yaml")
	spec := loadOpenAPISpecInfo(t, "checker/testdata/property_deprecation_anyof_spec.yaml")
	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)
	config := NewConfig(nil)
	changes := PropertyDeprecationCheck(d, osm, config)
	found := false
	for _, c := range changes {
		if c.GetId() == PropertyDeprecatedId && c.GetOperation() == "AnyOfSchema" && c.GetPath() == "anyOfProp" {
			found = true
			t.Logf("Found deprecated anyOf property: AnyOfSchema.anyOfProp")
		}
	}
	if !found {
		t.Errorf("Expected PropertyDeprecatedId for AnyOfSchema.anyOfProp in changes, got: %+v", changes)
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
