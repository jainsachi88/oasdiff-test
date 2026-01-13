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

	if diffReport.ComponentsDiff == nil || diffReport.ComponentsDiff.SchemasDiff == nil {
		println("[DeprecationCheck] No components diff or schemas diff found")
		return result
	}
	for schemaName, schemaDiff := range diffReport.ComponentsDiff.SchemasDiff.Modified {
		if schemaDiff.PropertiesDiff != nil && schemaDiff.PropertiesDiff.Modified != nil {
			traversePropertyDiffs(schemaName, "", schemaDiff.PropertiesDiff.Modified, &result, config, operationsSources)
		}
		// Always traverse composition branches even if PropertiesDiff.Modified is nil
		for _, composition := range []*diff.SubschemasDiff{schemaDiff.AllOfDiff, schemaDiff.OneOfDiff, schemaDiff.AnyOfDiff} {
			if composition != nil && composition.Modified != nil {
				for _, mod := range composition.Modified {
					if mod.Diff != nil && mod.Diff.PropertiesDiff != nil && mod.Diff.PropertiesDiff.Modified != nil {
						traversePropertyDiffs(schemaName, "", mod.Diff.PropertiesDiff.Modified, &result, config, operationsSources)
					}
				}
			}
		}
	}
	return result
}

// newPropertyApiChange creates a Change for property deprecation with safe dummy values

func traversePropertyDiffs(schemaName, parentPath string, propertyDiffs map[string]*diff.SchemaDiff, result *Changes, config *Config, operationsSources *diff.OperationsSourcesMap) {
	for propertyName, propertyDiff := range propertyDiffs {
		fullPath := propertyName
		if parentPath != "" {
			fullPath = parentPath + "." + propertyName
		}

		// Recursively check nested properties in composed schemas
		for idx, composition := range []*diff.SubschemasDiff{propertyDiff.AllOfDiff, propertyDiff.OneOfDiff, propertyDiff.AnyOfDiff} {
			if composition != nil && composition.Modified != nil {
				var compType string
				switch idx {
				case 0:
					compType = "allOf"
				case 1:
					compType = "oneOf"
				case 2:
					compType = "anyOf"
				}
				println("[DeprecationCheck] Traversing ", compType, " branch for property:", fullPath, "in schema:", schemaName)
				for _, mod := range composition.Modified {
					if mod.Diff != nil && mod.Diff.PropertiesDiff != nil && mod.Diff.PropertiesDiff.Modified != nil {
						traversePropertyDiffs(schemaName, fullPath, mod.Diff.PropertiesDiff.Modified, result, config, operationsSources)
					}
				}
			}
		}
		if propertyDiff.DeprecatedDiff != nil && propertyDiff.DeprecatedDiff.To == true && (propertyDiff.DeprecatedDiff.From == nil || propertyDiff.DeprecatedDiff.From == false) {
			println("[DeprecationCheck] Property deprecated:", fullPath, "in schema:", schemaName)
			*result = append(*result, NewApiChange(
				PropertyDeprecatedId,
				config,
				[]any{schemaName, fullPath},
				"",
				operationsSources,
				nil,
				schemaName,
				fullPath,
			))
		}
		// Recursively check nested object properties
		if propertyDiff.PropertiesDiff != nil && propertyDiff.PropertiesDiff.Modified != nil {
			println("[DeprecationCheck] Traversing nested object properties for:", fullPath, "in schema:", schemaName)
			traversePropertyDiffs(schemaName, fullPath, propertyDiff.PropertiesDiff.Modified, result, config, operationsSources)
		}
	}
}
