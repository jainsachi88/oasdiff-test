package checker

import (
	"testing"

	"github.com/oasdiff/oasdiff/diff"
	"github.com/stretchr/testify/require"
)

func TestRequestPropertyDeprecationCheck(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "testdata/request_property_deprecation_base.yaml")
	spec := loadOpenAPISpecInfo(t, "testdata/request_property_deprecation_spec.yaml")

	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)

	config := NewConfig(nil)
	changes := RequestPropertyDeprecationCheck(d, osm, config)

	found := false
	for _, c := range changes {
		if c.GetId() == RequestPropertyDeprecatedId {
			found = true
			t.Logf("Found deprecated request property: %+v", c)
		}
	}
	if !found {
		t.Errorf("Expected RequestPropertyDeprecatedId in changes, got: %+v", changes)
	}
}

func TestRequestPropertyDeprecationCheck_AllOf(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "testdata/request_property_deprecation_allof_base.yaml")
	spec := loadOpenAPISpecInfo(t, "testdata/request_property_deprecation_allof_spec.yaml")

	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)

	config := NewConfig(nil)
	changes := RequestPropertyDeprecationCheck(d, osm, config)

	found := false
	for _, c := range changes {
		if c.GetId() == RequestPropertyDeprecatedId {
			found = true
			t.Logf("Found deprecated request allOf property: %+v", c)
		}
	}
	if !found {
		t.Errorf("Expected RequestPropertyDeprecatedId in changes, got: %+v", changes)
	}
}

func TestRequestPropertyDeprecationCheck_NoDuplicates(t *testing.T) {
	base := loadOpenAPISpecInfo(t, "testdata/request_property_deprecation_allof_base.yaml")
	spec := loadOpenAPISpecInfo(t, "testdata/request_property_deprecation_allof_spec.yaml")

	d, osm, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), base, spec)
	require.NoError(t, err)

	config := NewConfig(nil)
	changes := RequestPropertyDeprecationCheck(d, osm, config)

	// Count occurrences of each property
	propCount := make(map[string]int)
	for _, c := range changes {
		if c.GetId() == RequestPropertyDeprecatedId {
			propCount[c.GetText(NewDefaultLocalizer())]++
		}
	}

	// Each property should only appear once
	for prop, count := range propCount {
		if count > 1 {
			t.Errorf("Property %s appears %d times, expected 1", prop, count)
		}
	}
}
