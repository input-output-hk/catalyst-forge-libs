// Valid repository configuration for testing
forgeVersion: "0.1.0"

tagging: {
	strategy: "monorepo"
}

phases: {
	build: {
		group:       1
		description: "Build artifacts"
		timeout:     "30m"
		required:    true
	}
	test: {
		group:       1
		description: "Run tests"
		timeout:     "20m"
		required:    true
	}
	deploy: {
		group:       2
		description: "Deploy to environment"
		timeout:     "15m"
		required:    false
	}
}

publishers: {
	docker: {
		type:      "docker"
		registry:  "ghcr.io"
		namespace: "test-org"
	}
	github: {
		type:       "github"
		repository: "test-org/test-repo"
	}
}
