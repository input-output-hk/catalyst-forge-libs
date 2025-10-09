package phases

// PhaseDefinition defines a pipeline phase with its execution group and metadata.
// Phases with the same group number execute in parallel.
// Groups are executed sequentially in ascending order.
#PhaseDefinition: {
	group:        int    // Execution group number (phases in same group run in parallel)
	description?: string // Optional description of phase purpose
	timeout?:     string // Optional timeout (e.g., "30m", "1h")
	required?:    bool   // Whether phase is required (default: false)
}
