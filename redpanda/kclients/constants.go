package kclients

// Compatibility level constants for Schema Registry
const (
	// DefaultCompatibilityLevel is the default compatibility level for Schema Registry subjects
	DefaultCompatibilityLevel = "BACKWARD"

	// CompatibilityBackward allows consumers using the new schema to read data produced with the previous schema
	CompatibilityBackward = "BACKWARD"

	// CompatibilityBackwardTransitive checks compatibility against all previous schema versions
	CompatibilityBackwardTransitive = "BACKWARD_TRANSITIVE"

	// CompatibilityForward allows consumers using the previous schema to read data produced with the new schema
	CompatibilityForward = "FORWARD"

	// CompatibilityForwardTransitive checks forward compatibility against all previous schema versions
	CompatibilityForwardTransitive = "FORWARD_TRANSITIVE"

	// CompatibilityFull requires both backward and forward compatibility
	CompatibilityFull = "FULL"

	// CompatibilityFullTransitive checks both backward and forward compatibility against all previous schema versions
	CompatibilityFullTransitive = "FULL_TRANSITIVE"

	// CompatibilityNone disables schema compatibility checking
	CompatibilityNone = "NONE"
)
