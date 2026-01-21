package checker

import (
	"github.com/oasdiff/oasdiff/diff"
)

const (
	RequestPropertyDeprecatedId              = "request-property-deprecated"
	RequestPropertyDeprecatedSunsetMissingId = "request-property-deprecated-sunset-missing"
	RequestPropertyDeprecatedParseId         = "request-property-deprecated-sunset-parse"
)

func RequestPropertyDeprecationCheck(diffReport *diff.Diff, operationsSources *diff.OperationsSourcesMap, config *Config) Changes {
	result := make(Changes, 0)
	if diffReport.PathsDiff == nil {
		return result
	}
	for path, pathItem := range diffReport.PathsDiff.Modified {
		if pathItem.OperationsDiff == nil {
			continue
		}
		for operation, operationItem := range pathItem.OperationsDiff.Modified {
			if operationItem.RequestBodyDiff == nil {
				continue
			}
			if operationItem.RequestBodyDiff.ContentDiff == nil {
				continue
			}
			// Track reported properties to avoid duplicates per operation
			reportedProperties := make(map[string]*diff.SchemaDiff)
			for _, mediaTypeDiff := range operationItem.RequestBodyDiff.ContentDiff.MediaTypeModified {
				if mediaTypeDiff.SchemaDiff == nil {
					continue
				}
				// Collect all deprecated properties, deduplicating as we go
				collectRequestDeprecatedProperties(mediaTypeDiff.SchemaDiff, reportedProperties)
			}
			// Report each unique property once
			for propertyName, propertyDiff := range reportedProperties {
				changeId, args := getRequestPropertyDeprecationId(propertyName, propertyDiff)
				result = append(result, NewApiChange(
					changeId,
					config,
					args,
					"",
					operationsSources,
					operationItem.Revision,
					operation,
					path,
				))
			}
		}
	}
	return result
}

func getRequestPropertyDeprecationId(propertyName string, propertyDiff *diff.SchemaDiff) (string, []any) {
	if propertyDiff == nil || propertyDiff.Revision == nil {
		return RequestPropertyDeprecatedSunsetMissingId, []any{propertyName}
	}
	sunset, ok := getSunset(propertyDiff.Revision.Extensions)
	if !ok {
		return RequestPropertyDeprecatedSunsetMissingId, []any{propertyName}
	}
	date, err := getSunsetDate(sunset)
	if err != nil {
		return RequestPropertyDeprecatedParseId, []any{propertyName, err}
	}
	return RequestPropertyDeprecatedId, []any{propertyName, date}
}

// collectRequestDeprecatedProperties collects all deprecated property names into the map
func collectRequestDeprecatedProperties(schemaDiff *diff.SchemaDiff, reportedProperties map[string]*diff.SchemaDiff) {
	if schemaDiff.PropertiesDiff != nil && schemaDiff.PropertiesDiff.Modified != nil {
		for propertyName, propertyDiff := range schemaDiff.PropertiesDiff.Modified {
			if propertyDiff.DeprecatedDiff == nil {
				continue
			}
			if propertyDiff.DeprecatedDiff.To != true {
				continue
			}
			if propertyDiff.DeprecatedDiff.From != nil && propertyDiff.DeprecatedDiff.From != false {
				continue
			}
			// Add to map (automatically deduplicates)
			if _, exists := reportedProperties[propertyName]; !exists {
				reportedProperties[propertyName] = propertyDiff
			}
		}
	}

	// Recursively check nested properties in composed schemas (allOf, oneOf, anyOf)
	for _, composition := range []*diff.SubschemasDiff{schemaDiff.AllOfDiff, schemaDiff.OneOfDiff, schemaDiff.AnyOfDiff} {
		if composition != nil && composition.Modified != nil {
			for _, mod := range composition.Modified {
				if mod.Diff != nil {
					collectRequestDeprecatedProperties(mod.Diff, reportedProperties)
				}
			}
		}
	}
}
