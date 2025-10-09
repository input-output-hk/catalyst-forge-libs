package schema

// Test RepoConfig schema with a valid example
_testRepoConfig: #RepoConfig & {
	forgeVersion: "0.1.0"
	tagging: {
		strategy: "monorepo"
	}
	phases: {
		test: {
			group:    1
			required: true
			timeout:  "30m"
		}
		build: {
			group:       2
			required:    true
			description: "Build phase"
		}
	}
	publishers: {
		dockerhub: {
			type:      "docker"
			registry:  "docker.io"
			namespace: "myorg"
		}
		github: {
			type:       "github"
			repository: "myorg/myrepo"
		}
	}
}
