package cfschema

import (
	"fmt"
)

// Expand replaces all Definition and Property JSON Pointer references with their content.
// This functionality removes the need for recursive logic when accessing Definitions and Properties.
// In unresolved form nested properties are not allowed, instead nested properties use a '$ref' JSON Pointer to reference a definition.
// See https://docs.aws.amazon.com/cloudformation-cli/latest/userguide/resource-type-schema.html#schema-properties-properties.
func (r *Resource) Expand() error {
	if r == nil {
		return nil
	}

	err := r.ResolveProperties(r.Definitions)

	if err != nil {
		return fmt.Errorf("error expanding Resource (%s) Definitions: %w", *r.TypeName, err)
	}

	err = r.ResolveProperties(r.Properties)

	if err != nil {
		return fmt.Errorf("error expanding Resource (%s) Properties: %w", *r.TypeName, err)
	}

	return nil
}

// ResolveProperties resolves all References in a top-level name-to-property map.
// In theory unresolved form nested properties are not allowed but in practice they do occur,
// so support arbitrarily deeply nested references.
func (r *Resource) ResolveProperties(properties map[string]*Property) error {
	for propertyName, property := range properties {
		// For example:
		// "Configuration": {
		//   "$ref": "#/definitions/ClusterConfiguration"
		// },
		resolved, err := r.ResolveProperty(property)

		if err != nil {
			return fmt.Errorf("error resolving %s: %w", propertyName, err)
		}

		if resolved {
			continue
		}

		switch property.Type.String() {
		case PropertyTypeArray:
			// For example:
			// "DefaultCapacityProviderStrategy": {
			//   "type": "array",
			//   "items": {
			//     "$ref": "#/definitions/CapacityProviderStrategyItem"
			//   }
			// },
			_, err = r.ResolveProperty(property.Items)

			if err != nil {
				return fmt.Errorf("error resolving %s Items: %w", propertyName, err)
			}

		case PropertyTypeObject:
			// For example:
			// "ClusterConfiguration": {
			//   "type": "object",
			//   "properties": {
			//     "ExecuteCommandConfiguration": {
			//       "$ref": "#/definitions/ExecuteCommandConfiguration"
			//     }
			//   }
			// },
			err = r.ResolveProperties(property.Properties)

			if err != nil {
				return fmt.Errorf("error resolving %s Properties: %w", propertyName, err)
			}

			// For example:
			// "LambdaFunctionRecipeSource": {
			//   "type": "object",
			//   "properties": {
			//     "ComponentDependencies": {
			//       "type": "object",
			//       "patternProperties": {
			//         "": {
			//           "$ref": "#/definitions/ComponentDependencyRequirement"
			//         }
			//       }
			//     }
			//   }
			// },
			err = r.ResolveProperties(property.PatternProperties)

			if err != nil {
				return fmt.Errorf("error resolving %s PatternProperties: %w", propertyName, err)
			}

		case "":
			if len(property.Properties) > 0 {
				// For example:
				// "PresignedUrlConfig": {
				//   "properties": {
				//     "RoleArn": {
				//       "$ref": "#/definitions/RoleArn"
				//     },
				//     "ExpiresInSec": {
				//       "$ref": "#/definitions/ExpiresInSec"
				//     }
				//   }
				// },
				err = r.ResolveProperties(property.Properties)

				if err != nil {
					return fmt.Errorf("error resolving %s Properties: %w", propertyName, err)
				}
			}
		}
	}

	return nil
}

// ResolveProperty resolves any Reference (JSON Pointer) in a Property.
// Returns whether a Reference was resolved.
func (r *Resource) ResolveProperty(property *Property) (bool, error) {
	if property != nil && property.Ref != nil {
		defaultValue := property.Default
		description := property.Description
		ref := property.Ref
		resolution, err := r.ResolveReference(*ref)

		if err != nil {
			return false, err
		}

		*property = *resolution

		// Ensure that any default value is not lost.
		if defaultValue != nil {
			property.Default = defaultValue
		}

		// Prefer any description from the unresolved property.
		if description != nil && *description != "" {
			property.Description = description
		}

		return true, nil
	}

	return false, nil
}
