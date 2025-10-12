// Valid project configuration for testing
name: "test-project"

phases: {
	build: {
		steps: [
			{
				name:   "compile"
				action: "earthly"
				target: "+build"
			},
		]
	}
	test: {
		steps: [
			{
				name:    "unit-tests"
				action:  "earthly"
				target:  "+test"
				timeout: "10m"
			},
		]
	}
}

artifacts: {
	"api-server": {
		type: "container"
		ref:  "api-server:latest"
		producer: {
			type:   "earthly"
			target: "+docker"
		}
		publishers: ["docker"]
	}
	binary: {
		type: "binary"
		name: "cli"
		producer: {
			type:   "earthly"
			target: "+build"
		}
		publishers: ["github"]
	}
}

release: {
	on: [
		{branch: "main"},
		{tag: true},
	]
}
