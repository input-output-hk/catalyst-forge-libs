package schema

// Test ProjectConfig schema with a valid example
_testProjectConfig: #ProjectConfig & {
	name: "my-service"
	phases: {
		test: {
			steps: [{
				name:   "run-tests"
				action: "earthly"
				target: "+test"
			}]
		}
		build: {
			steps: [{
				name:    "build-image"
				action:  "earthly"
				target:  "+docker"
				timeout: "20m"
			}]
		}
	}
	artifacts: {
		"service-image": {
			type: "container"
			ref:  "myorg/my-service:latest"
			producer: {
				type:   "earthly"
				target: "+docker"
			}
			publishers: ["dockerhub"]
		}
		"cli-binary": {
			type: "binary"
			name: "my-cli"
			producer: {
				type:   "earthly"
				target: "+build"
			}
			publishers: ["github"]
		}
	}
	release: {
		on: [{
			branch: "main"
		}, {
			tag: true
		}]
	}
	deploy: {
		resources: [{
			apiVersion: "apps/v1"
			kind:       "Deployment"
			metadata: {
				name: "my-service"
			}
			spec: {
				replicas: 3
			}
		}]
	}
}
