module github.com/input-output-hk/catalyst-forge-libs/secrets/providers/memory

go 1.22.0

require (
	github.com/input-output-hk/catalyst-forge-libs/secrets/core v0.0.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Local replace for development
replace github.com/input-output-hk/catalyst-forge-libs/secrets/core => ../../core
