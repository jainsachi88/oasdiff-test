package checker

import (
	"github.com/oasdiff/oasdiff/diff"
)

const (
	// Property deprecation change IDs
	PropertyReactivatedId             = "property-reactivated"
	PropertyDeprecatedSunsetMissingId = "property-deprecated-sunset-missing"
	PropertySunsetDateTooSmallId      = "property-sunset-date-too-small"
	PropertyDeprecatedId              = "property-deprecated"
)

// PropertyDeprecationCheck checks for deprecated properties in schemas
func PropertyDeprecationCheck(diffReport *diff.Diff, operationsSources *diff.OperationsSourcesMap, config *Config) Changes {
	result := make(Changes, 0)
	println("[DeprecationCheck] inside function ")
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
						traversePropertyDiffs(schemaName, operation, path, schemaDiff.PropertiesDiff.Modified, &result, config, operationsSources)
					}
					// Recursively check nested properties in composed schemas in allOf, oneOf, anyOf branches
					for _, composition := range []*diff.SubschemasDiff{schemaDiff.AllOfDiff, schemaDiff.OneOfDiff, schemaDiff.AnyOfDiff} {
						if composition != nil && composition.Modified != nil {
							for _, mod := range composition.Modified {
								if mod.Diff != nil && mod.Diff.PropertiesDiff != nil && mod.Diff.PropertiesDiff.Modified != nil {
									traversePropertyDiffs(schemaName, operation, path, mod.Diff.PropertiesDiff.Modified, &result, config, operationsSources)
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

func traversePropertyDiffs(schemaName, operation, endpointPath string, propertyDiffs map[string]*diff.SchemaDiff, result *Changes, config *Config, operationsSources *diff.OperationsSourcesMap) {
	for propertyName, propertyDiff := range propertyDiffs {
		// Only use propertyName and nested property path, not endpointPath prefix
		fullPath := propertyName
		if propertyDiff.DeprecatedDiff != nil && propertyDiff.DeprecatedDiff.To == true && (propertyDiff.DeprecatedDiff.From == nil || propertyDiff.DeprecatedDiff.From == false) {
			*result = append(*result, NewApiChange(
				PropertyDeprecatedId,
				config,
				[]any{schemaName, fullPath},
				"",
				operationsSources,
				nil,
				operation,
				endpointPath,
			))
		}
		// Recursively check nested object properties
		if propertyDiff.PropertiesDiff != nil && propertyDiff.PropertiesDiff.Modified != nil {
			traversePropertyDiffs(schemaName, operation, endpointPath, propertyDiff.PropertiesDiff.Modified, result, config, operationsSources)
		}
	}
}
