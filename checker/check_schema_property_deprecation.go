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

// PropertyDeprecationCheck checks for deprecated properties in schemas
func PropertyDeprecationCheck(diffReport *diff.Diff, operationsSources *diff.OperationsSourcesMap, config *Config) Changes {
	result := make(Changes, 0)
	if diffReport.PathsDiff == nil {
		return result
	}
	for path, pathItem := range diffReport.PathsDiff.Modified {
		if pathItem.OperationsDiff == nil {
			continue
		}
		for operation := range pathItem.OperationsDiff.Modified {
			op := pathItem.Revision.GetOperation(operation)
			if op == nil {
				continue
			}
			// For each operation, check all schemas in ComponentsDiff
			if diffReport.ComponentsDiff != nil && diffReport.ComponentsDiff.SchemasDiff != nil {
				for schemaName, schemaDiff := range diffReport.ComponentsDiff.SchemasDiff.Modified {
					if schemaDiff.PropertiesDiff != nil && schemaDiff.PropertiesDiff.Modified != nil {
						traversePropertyDiffs(schemaName, operation, path, schemaDiff.PropertiesDiff.Modified, &result, config, operationsSources, op)
					}
					// Recursively check nested properties in composed schemas in allOf, oneOf, and anyOf
					for _, composition := range []*diff.SubschemasDiff{schemaDiff.AllOfDiff, schemaDiff.OneOfDiff, schemaDiff.AnyOfDiff} {
						if composition != nil && composition.Modified != nil {
							for _, mod := range composition.Modified {
								if mod.Diff != nil && mod.Diff.PropertiesDiff != nil && mod.Diff.PropertiesDiff.Modified != nil {
									traversePropertyDiffs(schemaName, operation, path, mod.Diff.PropertiesDiff.Modified, &result, config, operationsSources, op)
								}
							}
						}
					}
				}
			}
		}
	}
	return result
}

func traversePropertyDiffs(schemaName, operation, endpointPath string, propertyDiffs map[string]*diff.SchemaDiff, result *Changes, config *Config, operationsSources *diff.OperationsSourcesMap, op *openapi3.Operation) {
	for propertyName, propertyDiff := range propertyDiffs {
		// Only use propertyName and nested property path, not endpointPath prefix
		fullPath := propertyName

		if propertyDiff.DeprecatedDiff != nil && propertyDiff.DeprecatedDiff.To == true && (propertyDiff.DeprecatedDiff.From == nil || propertyDiff.DeprecatedDiff.From == false) {
			var extensions map[string]interface{}
			if propertyDiff.Revision != nil {
				extensions = propertyDiff.Revision.Extensions
			}
			stability, err := getStabilityLevel(extensions)
			if err != nil {
				// skip or handle as needed
				continue
			}
			deprecationDays := getDeprecationDays(config, stability)
			sunset, ok := getSunset(extensions)
			if !ok {
				if deprecationDays > 0 {
					*result = append(*result, NewApiChange(
						PropertyDeprecatedSunsetMissingId,
						config,
						[]any{schemaName, fullPath},
						"",
						operationsSources,
						op,
						operation,
						endpointPath,
					))
				}
				continue
			}
			_, err = getSunsetDate(sunset)
			if err != nil {
				*result = append(*result, NewApiChange(
					PropertyDeprecatedSunsetParseId,
					config,
					[]any{schemaName, fullPath, err},
					"",
					operationsSources,
					op,
					operation,
					endpointPath,
				))
				continue
			}
			*result = append(*result, NewApiChange(
				PropertyDeprecatedId,
				config,
				[]any{schemaName, fullPath, sunset},
				"",
				operationsSources,
				op,
				operation,
				endpointPath,
			))
		}
		// Recursively check nested object properties
		if propertyDiff.PropertiesDiff != nil && propertyDiff.PropertiesDiff.Modified != nil {
			traversePropertyDiffs(schemaName, operation, endpointPath, propertyDiff.PropertiesDiff.Modified, result, config, operationsSources, op)
		}
	}
}
