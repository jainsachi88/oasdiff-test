package checker

import (
	"github.com/getkin/kin-openapi/openapi3"
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
			reportedProperties := make(map[string]bool)
			for _, mediaTypeDiff := range operationItem.RequestBodyDiff.ContentDiff.MediaTypeModified {
				if mediaTypeDiff.SchemaDiff == nil {
					continue
				}
				checkRequestPropertyDeprecation(mediaTypeDiff.SchemaDiff, &result, config, operationsSources, operationItem.Revision, operation, path, reportedProperties)
			}
		}
	}
	return result
}

func checkRequestPropertyDeprecation(schemaDiff *diff.SchemaDiff, result *Changes, config *Config, operationsSources *diff.OperationsSourcesMap, revision *openapi3.Operation, operation string, path string, reportedProperties map[string]bool) {
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
						RequestPropertyDeprecatedSunsetMissingId,
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
					RequestPropertyDeprecatedParseId,
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
				RequestPropertyDeprecatedId,
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
					checkRequestPropertyDeprecation(mod.Diff, result, config, operationsSources, revision, operation, path, reportedProperties)
				}
			}
		}
	}
}
