# CI

Catalyst uses a central developer platform called Catalyst Forge.
This platform ships with an all-in-one CI system that works by scanning the repository for blueprint files and then executing
Earthly targets in a predetermined order.

## GitHub Action

The CI workflow can be imported using a single reusable workflow:

```yaml
name: CI

on:
  push:
    branches: [master] # Ensure this matches the default branch
    tags: ['**']
  pull_request:

permissions:
  id-token: write
  contents: write
  packages: write
  pull-requests: write

jobs:
  ci:
    uses: input-output-hk/catalyst-forge/.github/workflows/ci.yml@ci/v1.10.0
    with:
      forge_version: 0.21.0
```

## Adding a project

To add a new project to the CI run, create a `blueprint.cue` at the root of the project directory.
At a minimum, the blueprint must contain a unique project name:

```cue
project: {
	name: "<project_name>"
}
```

Then, create an `Earthfile` that specifies what should be executed during each CI phase.
By default, the CI executes the following phases in order:

- **check**: Used for running fast checks like linting, formatting, etc.
- **build**: Used to build the main package or program
- **package**: Used to package the project into a Docker image (uses `SAVE IMAGE`)
- **test**: Used to run unit + integration tests

### Example

Here's an example for a Go support module (no `main` package):

```
VERSION 0.8

deps:
    FROM golang:1.24.2-bookworm

    WORKDIR /work

    RUN mkdir -p /go/cache && mkdir -p /go/modcache
    ENV GOCACHE=/go/cache
    ENV GOMODCACHE=/go/modcache
    CACHE --persist --sharing shared /go

    RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.3.1

    COPY go.mod go.sum .
    RUN go mod download

src:
    FROM +deps

    CACHE --persist --sharing shared /go

    COPY . .

    RUN go generate ./...

    SAVE ARTIFACT . src

check:
    FROM +src

    RUN golangci-lint run ./...

test:
    FROM +src

    RUN go test ./...
```

### Validating Earthly targets

Earthly targets can be validated to work locally _before_ pushing to CI:

```bash
# Run the `check` target
earthly --config "" +check
```

## Integration Tests with Testcontainers

Most of our integration tests use `testcontainers` for bringing up short-lived containers for testing purposes.
By default, these will _not_ work without some additional modifications to the `test` target:

```
test:
    FROM earthly/dind:ubuntu-24.04-docker-27.3.1-1

    WORKDIR /work

    COPY --dir test .

    RUN go test -tags=integration -v ./...
```

This will ensure that DIND is available to start containers.
You may need to copy in additional dependencies from the `src` target depending on how the integration tests work.
When testing this target, you _must_ run it as privileged:

```bash
earthly -P --config "" +test
```

Additionally, the `blueprint.cue` must be configured to run the `test` target as privileged during CI runs:

```cue
project: {
	name: "<project_name>"
	ci: targets: {
		test: privileged: true
	}
}
```

All targets are allowed to have any number of characters/integers/special characters after their name.
For example, you may run unit tests in:

```
test-unit:
    # Run unit tests
```

And then integration tests in:

```
test-integration:
    # Run integration tests
```

The CI system will pick up and execute _both_ of these targets during the `test` phase.
Note that in the above example you'd need to change the previously given `blueprint.cue` to reference the `test-integration` target
instead of the `test` target.