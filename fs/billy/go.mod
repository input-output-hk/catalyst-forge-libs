module github.com/input-output-hk/catalyst-forge-libs/fs/billy

go 1.24.2

replace (
	github.com/input-output-hk/catalyst-forge-libs/fs/core => ../core
	github.com/input-output-hk/catalyst-forge-libs/fs/fstest => ../fstest
)

require (
	github.com/go-git/go-billy/v5 v5.5.0
	github.com/input-output-hk/catalyst-forge-libs/fs/core v0.0.0
	github.com/input-output-hk/catalyst-forge-libs/fs/fstest v0.0.0-00010101000000-000000000000
)

require (
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	golang.org/x/sys v0.12.0 // indirect
)
