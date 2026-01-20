package checker

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/oasdiff/diff"
)

const (
	// Property deprecation change IDs
	PropertyDeprecatedSunsetMissingId = "property-deprecated-sunset-missing"
	PropertyDeprecatedSunsetParseId   = "property-deprecated-sunset-parse"
	PropertyDeprecatedId              = "property-deprecated"

	// Request property deprecation change IDs
	RequestPropertyDeprecatedId              = "request-property-deprecated"
	RequestPropertyDeprecatedSunsetMissingId = "request-property-deprecated-sunset-missing"
	RequestPropertyDeprecatedParseId         = "request-property-deprecated-sunset-parse"
)

// PropertyDeprecationCheck checks for deprecated properties in schemas
func PropertyDeprecationCheck(diffReport *diff.Diff, operationsSources *diff.OperationsSourcesMap, config *Config) Changes {
	result := make(Changes, 0)
	seen := make(map[string]bool) // Track seen changes to avoid duplicates

	// Check schema component property deprecations (report once per schema, not per endpoint)
	if diffReport.ComponentsDiff != nil && diffReport.ComponentsDiff.SchemasDiff != nil {
		for schemaName, schemaDiff := range diffReport.ComponentsDiff.SchemasDiff.Modified {
			if schemaDiff.PropertiesDiff != nil && schemaDiff.PropertiesDiff.Modified != nil {
				traversePropertyDiffs(schemaName, "", "", schemaDiff.PropertiesDiff.Modified, &result, config, operationsSources, nil, seen, false)
			}
			// Recursively check nested properties in composed schemas in allOf, oneOf, and anyOf
			for _, composition := range []*diff.SubschemasDiff{schemaDiff.AllOfDiff, schemaDiff.OneOfDiff, schemaDiff.AnyOfDiff} {
				if composition != nil && composition.Modified != nil {
					for _, mod := range composition.Modified {
						if mod.Diff != nil && mod.Diff.PropertiesDiff != nil && mod.Diff.PropertiesDiff.Modified != nil {
							traversePropertyDiffs(schemaName, "", "", mod.Diff.PropertiesDiff.Modified, &result, config, operationsSources, nil, seen, false)
						}
					}
				}
			}
		}
	}

	// Check request body property deprecations (per endpoint)
	if diffReport.PathsDiff != nil {
		for path, pathItem := range diffReport.PathsDiff.Modified {
			if pathItem.OperationsDiff == nil {
				continue
			}
			for operation, operationDiff := range pathItem.OperationsDiff.Modified {
				op := pathItem.Revision.GetOperation(operation)
				if op == nil {
					continue
				}
				if operationDiff.RequestBodyDiff != nil && operationDiff.RequestBodyDiff.ContentDiff != nil {
					for _, mediaTypeDiff := range operationDiff.RequestBodyDiff.ContentDiff.MediaTypeModified {
						if mediaTypeDiff.SchemaDiff == nil {
							continue
						}
						if mediaTypeDiff.SchemaDiff.PropertiesDiff != nil && mediaTypeDiff.SchemaDiff.PropertiesDiff.Modified != nil {
							traversePropertyDiffs("", operation, path, mediaTypeDiff.SchemaDiff.PropertiesDiff.Modified, &result, config, operationsSources, op, seen, true)
						}
						// Check nested properties in allOf, oneOf, anyOf
						for _, composition := range []*diff.SubschemasDiff{mediaTypeDiff.SchemaDiff.AllOfDiff, mediaTypeDiff.SchemaDiff.OneOfDiff, mediaTypeDiff.SchemaDiff.AnyOfDiff} {
							if composition != nil && composition.Modified != nil {
								for _, mod := range composition.Modified {
									if mod.Diff != nil && mod.Diff.PropertiesDiff != nil && mod.Diff.PropertiesDiff.Modified != nil {
										traversePropertyDiffs("", operation, path, mod.Diff.PropertiesDiff.Modified, &result, config, operationsSources, op, seen, true)
									}
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

func traversePropertyDiffs(schemaName, operation, endpointPath string, propertyDiffs map[string]*diff.SchemaDiff, result *Changes, config *Config, operationsSources *diff.OperationsSourcesMap, op *openapi3.Operation, seen map[string]bool, isRequest bool) {
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

			// Build unique key for deduplication
			var changeId string
			if isRequest {
				if !ok {
					changeId = RequestPropertyDeprecatedSunsetMissingId
				} else {
					changeId = RequestPropertyDeprecatedId
				}
				key := changeId + "|" + endpointPath + "|" + operation + "|" + fullPath
				if seen[key] {
					continue
				}
				seen[key] = true
			} else {
				if !ok {
					changeId = PropertyDeprecatedSunsetMissingId
				} else {
					changeId = PropertyDeprecatedId
				}
				key := changeId + "|" + schemaName + "|" + fullPath
				if seen[key] {
					continue
				}
				seen[key] = true
			}

			if !ok {
				if deprecationDays > 0 {
					if isRequest {
						*result = append(*result, NewApiChange(
							RequestPropertyDeprecatedSunsetMissingId,
							config,
							[]any{fullPath},
							"",
							operationsSources,
							op,
							operation,
							endpointPath,
						))
					} else {
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
				}
				continue
			}
			_, err = getSunsetDate(sunset)
			if err != nil {
				if isRequest {
					*result = append(*result, NewApiChange(
						RequestPropertyDeprecatedParseId,
						config,
						[]any{fullPath, err},
						"",
						operationsSources,
						op,
						operation,
						endpointPath,
					))
				} else {
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
				}
				continue
			}
			if isRequest {
				*result = append(*result, NewApiChange(
					RequestPropertyDeprecatedId,
					config,
					[]any{fullPath, sunset},
					"",
					operationsSources,
					op,
					operation,
					endpointPath,
				))
			} else {
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
		}
		// Recursively check nested object properties
		if propertyDiff.PropertiesDiff != nil && propertyDiff.PropertiesDiff.Modified != nil {
			traversePropertyDiffs(schemaName, operation, endpointPath, propertyDiff.PropertiesDiff.Modified, result, config, operationsSources, op, seen, isRequest)
		}
	}
}
