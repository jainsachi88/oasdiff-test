package checker

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/oasdiff/diff"
)

const (
	ResponsePropertyDeprecatedId              = "response-property-deprecated"
	ResponsePropertyDeprecatedSunsetMissingId = "response-property-deprecated-sunset-missing"
	ResponsePropertyDeprecatedParseId         = "response-property-deprecated-sunset-parse"
)

func ResponsePropertyDeprecationCheck(diffReport *diff.Diff, operationsSources *diff.OperationsSourcesMap, config *Config) Changes {
	result := make(Changes, 0)
	if diffReport.PathsDiff == nil {
		return result
	}
	for path, pathItem := range diffReport.PathsDiff.Modified {
		if pathItem.OperationsDiff == nil {
			continue
		}
		for operation, operationItem := range pathItem.OperationsDiff.Modified {
			if operationItem.ResponsesDiff == nil {
				continue
			}
			if operationItem.ResponsesDiff.Modified == nil {
				continue
			}
			// Track reported properties to avoid duplicates per operation
			reportedProperties := make(map[string]bool)
			for _, responseDiff := range operationItem.ResponsesDiff.Modified {
				if responseDiff.ContentDiff == nil {
					continue
				}
				for _, mediaTypeDiff := range responseDiff.ContentDiff.MediaTypeModified {
					if mediaTypeDiff.SchemaDiff == nil {
						continue
					}
					checkResponsePropertyDeprecation(mediaTypeDiff.SchemaDiff, &result, config, operationsSources, operationItem.Revision, operation, path, reportedProperties)
				}
			}
		}
	}
	return result
}

func checkResponsePropertyDeprecation(schemaDiff *diff.SchemaDiff, result *Changes, config *Config, operationsSources *diff.OperationsSourcesMap, revision *openapi3.Operation, operation string, path string, reportedProperties map[string]bool) {
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
			// Skip if already reported
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
						ResponsePropertyDeprecatedSunsetMissingId,
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
					ResponsePropertyDeprecatedParseId,
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
				ResponsePropertyDeprecatedId,
				config,
				[]any{propertyName},
				"",
				operationsSources,
				revision,
				operation,
				path,
			))
		}
	}

	// Recursively check nested properties in composed schemas (allOf, oneOf, anyOf)
	for _, composition := range []*diff.SubschemasDiff{schemaDiff.AllOfDiff, schemaDiff.OneOfDiff, schemaDiff.AnyOfDiff} {
		if composition != nil && composition.Modified != nil {
			for _, mod := range composition.Modified {
				if mod.Diff != nil {
					checkResponsePropertyDeprecation(mod.Diff, result, config, operationsSources, revision, operation, path, reportedProperties)
				}
			}
		}
	}
}
