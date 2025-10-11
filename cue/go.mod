module github.com/input-output-hk/catalyst-forge-libs/cue

go 1.24.2

require (
	cuelang.org/go v0.12.0
	github.com/input-output-hk/catalyst-forge-libs/errors v0.0.0
	github.com/input-output-hk/catalyst-forge-libs/fs/core v0.0.0
	github.com/input-output-hk/catalyst-forge-libs/fs/billy v0.0.0
)

replace github.com/input-output-hk/catalyst-forge-libs/errors => ../errors

replace github.com/input-output-hk/catalyst-forge-libs/fs/core => ../fs/core

replace github.com/input-output-hk/catalyst-forge-libs/fs/billy => ../fs/billy
