# Monolithic Builder for Konflux

A monolithic builder implementation for Red Hat Konflux that consolidates multiple
Tekton pipeline tasks (`init`, `clone-repository`, `prefetch-dependencies`,
`build-container`, and `build-image-index`) into two efficient Go-based tasks for
improved performance and maintainability.

## Building

### Local Development
```bash
# Build the binary locally
make build

# Clean build artifacts
make clean

# Run tests
make test

# Run all tests including Ginkgo and coverage
make test-all
```

### Container Image
```bash
# Build the unified container image
make docker-build

# Or build manually
docker build -t quay.io/yftacherzog-konflux/monolithic-builder:latest .
```

The unified image supports both tasks through command detection:
- Default behavior: `build-container`
- Environment variable: `MONOLITHIC_COMMAND=build-image-index`
- Symlink: `/usr/local/bin/build-image-index`

## Testing

This project uses [Ginkgo](https://onsi.github.io/ginkgo/) for testing.

### Test Suites

```bash
# Run tests through Go's test runner
make test

# Run Ginkgo tests with enhanced reporting
make test-ginkgo

# Run tests with coverage report
make test-coverage

# Run all test suites
make test-all
```

## Working Examples

For complete working examples of the PipelineRun and Task definitions, see:
[testrepo commit d8201e7](https://github.com/yftacherzog/testrepo/commit/d8201e7f220df3f57d77dc3e680e45eb623debd8)

This commit includes:
- `.tekton/testrepo-monolithic-test.yaml` - Complete PipelineRun example
- `tasks/monolithic-build-container-task.yaml` - Build container task definition
- `tasks/monolithic-build-image-index-task.yaml` - Build image index task definition

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
