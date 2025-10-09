package phases

// PhaseDefinition defines a pipeline phase with its execution group and metadata.
// Phases with the same group number execute in parallel.
// Groups are executed sequentially in ascending order.
#PhaseDefinition: {
	// Execution group number (phases in same group run in parallel)
	group: int
	// Optional description of phase purpose
	description?: string
	// Optional timeout (e.g., "30m", "1h")
	timeout?: string
	// Whether phase is required (default: false)
	required?: bool
}
