module github.com/input-output-hk/catalyst-forge-libs/config

go 1.24.2

require (
	github.com/input-output-hk/catalyst-forge-libs/errors v0.0.0
	github.com/input-output-hk/catalyst-forge-libs/fs/core v0.0.0
	github.com/input-output-hk/catalyst-forge-libs/schema v0.0.0
)

require github.com/Masterminds/semver/v3 v3.3.0 // indirect

replace github.com/input-output-hk/catalyst-forge-libs/schema => ../schemas

replace github.com/input-output-hk/catalyst-forge-libs/cue => ../cue

replace github.com/input-output-hk/catalyst-forge-libs/errors => ../errors

replace github.com/input-output-hk/catalyst-forge-libs/fs/core => ../fs/core
