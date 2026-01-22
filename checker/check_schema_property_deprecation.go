package checker

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/oasdiff/diff"
)

const (
	// Property deprecation change IDs
	PropertyReactivatedId             = "property-reactivated"
	PropertyDeprecatedSunsetMissingId = "property-deprecated-sunset-missing"
	PropertyDeprecatedSunsetParseId   = "property-deprecated-sunset-parse"
	PropertyDeprecatedId              = "property-deprecated"
)

// PropertyDeprecationCheck checks for deprecated properties in a schema
func PropertyDeprecationCheck(schemaDiff *diff.SchemaDiff, parentPath string, result *Changes, config *Config, operationsSources *diff.OperationsSourcesMap, revision *openapi3.Operation, operation string, path string, reportedProperties map[string]bool) {
	if schemaDiff.PropertiesDiff != nil && schemaDiff.PropertiesDiff.Modified != nil {
		for propertyName, propertyDiff := range schemaDiff.PropertiesDiff.Modified {
			fullPath := propertyName
			if parentPath != "" {
				fullPath = parentPath + "/" + propertyName
			}

			if propertyDiff.DeprecatedDiff != nil && propertyDiff.DeprecatedDiff.To == true && (propertyDiff.DeprecatedDiff.From == nil || propertyDiff.DeprecatedDiff.From == false) {
				// Skip if already reported for this operation - use just propertyName to dedupe across allOf/oneOf/anyOf
				if reportedProperties[propertyName] {
					continue
				}
				reportedProperties[propertyName] = true

				var extensions map[string]interface{}
				if propertyDiff.Revision != nil {
					extensions = propertyDiff.Revision.Extensions
				}
				stability, err := getStabilityLevel(extensions)
				if err != nil {
					continue
				}
				deprecationDays := getDeprecationDays(config, stability)
				sunset, ok := getSunset(extensions)
				if !ok {
					if deprecationDays > 0 {
						*result = append(*result, NewApiChange(
							PropertyDeprecatedSunsetMissingId,
							config,
							[]any{propertyName},
							"",
							operationsSources,
							revision,
							operation,
							path,
						))
					}
					continue
				}
				_, err = getSunsetDate(sunset)
				if err != nil {
					*result = append(*result, NewApiChange(
						PropertyDeprecatedSunsetParseId,
						config,
						[]any{propertyName, err},
						"",
						operationsSources,
						revision,
						operation,
						path,
					))
					continue
				}
				*result = append(*result, NewApiChange(
					PropertyDeprecatedId,
					config,
					[]any{propertyName},
					"",
					operationsSources,
					revision,
					operation,
					path,
				))
			}
			// Recursively check nested object properties
			if propertyDiff.PropertiesDiff != nil && propertyDiff.PropertiesDiff.Modified != nil {
				PropertyDeprecationCheck(propertyDiff, fullPath, result, config, operationsSources, revision, operation, path, reportedProperties)
			}
		}
	}

	// Recursively check nested properties in composed schemas (allOf, oneOf, anyOf)
	for _, composition := range []*diff.SubschemasDiff{schemaDiff.AllOfDiff, schemaDiff.OneOfDiff, schemaDiff.AnyOfDiff} {
		if composition != nil && composition.Modified != nil {
			for _, mod := range composition.Modified {
				if mod.Diff != nil {
					PropertyDeprecationCheck(mod.Diff, parentPath, result, config, operationsSources, revision, operation, path, reportedProperties)
				}
			}
		}
	}
}
