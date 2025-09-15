package image

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/konflux-ci/monolithic-builder/pkg/exec"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("BuildAndPush Integration", func() {
	var (
		ctx        context.Context
		logger     *zap.Logger
		mockRunner *exec.MockCommandRunner
		config     *BuildConfig
	)

	BeforeEach(func() {
		ctx = context.Background()
		logger = zap.NewNop() // No-op logger for tests
		mockRunner = exec.NewMockCommandRunner()

		config = &BuildConfig{
			ImageURL:   "quay.io/test/image:latest",
			Dockerfile: "./Dockerfile",
			Context:    "/workspace/source",
			TLSVerify:  true,
			BuildArgs:  []string{"GO_VERSION=1.21", "DEBUG=false"},
			CommitSHA:  "abc123def456",
		}
	})

	Context("when building and pushing successfully", func() {
		BeforeEach(func() {
			// Mock successful digest retrieval
			digestResponse := map[string]interface{}{
				"Digest": "sha256:abcdef123456789",
			}
			digestJSON, _ := json.Marshal(digestResponse)
			mockRunner.SetOutput(
				"skopeo", digestJSON, "inspect", "docker://quay.io/test/image:latest",
			)
		})

		It("should return complete build result with image digest", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ImageURL).To(Equal("quay.io/test/image:latest"))
			Expect(result.ImageDigest).To(Equal("sha256:abcdef123456789"))
		})

		It("should execute build and push operations", func() {
			_, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).NotTo(HaveOccurred())

			// Verify that build and push operations occurred (behavior, not specific commands)
			commands := mockRunner.GetExecutedCommands()
			Expect(commands).To(HaveLen(3)) // build, push, inspect

			// Verify build operation uses unshare wrapper
			Expect(commands[0][0]).To(Equal("unshare"))

			// Verify push operation
			Expect(commands[1][0]).To(Equal("buildah"))
			Expect(commands[1][1]).To(Equal("push"))
		})

		It("should process build arguments correctly", func() {
			// Test with specific build args to ensure they're handled
			config.BuildArgs = []string{"GO_VERSION=1.21", "DEBUG=false", "CUSTOM_ARG=test"}

			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify build args are passed to the build command (minimal verification)
			commands := mockRunner.GetExecutedCommands()
			buildCmd := commands[0] // The unshare/build command
			buildCmdStr := strings.Join(buildCmd, " ")

			// Just verify that build args are included somewhere in the command
			Expect(buildCmdStr).To(ContainSubstring("GO_VERSION=1.21"))
			Expect(buildCmdStr).To(ContainSubstring("DEBUG=false"))
			Expect(buildCmdStr).To(ContainSubstring("CUSTOM_ARG=test"))
		})

		It("should include commit metadata when provided", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify commit SHA is included in build metadata (minimal verification)
			commands := mockRunner.GetExecutedCommands()
			buildCmd := commands[0]
			buildCmdStr := strings.Join(buildCmd, " ")
			Expect(buildCmdStr).To(ContainSubstring("io.konflux.commit=abc123def456"))
		})
	})

	Context("when TLS verification is disabled", func() {
		BeforeEach(func() {
			config.TLSVerify = false

			// Mock successful response with TLS disabled
			digestResponse := map[string]interface{}{
				"Digest": "sha256:abcdef123456789",
			}
			digestJSON, _ := json.Marshal(digestResponse)
			mockRunner.SetOutput(
				"skopeo",
				digestJSON,
				"inspect",
				"--tls-verify=false",
				"docker://quay.io/test/image:latest",
			)
		})

		It("should successfully build and push with TLS verification disabled", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ImageURL).To(Equal("quay.io/test/image:latest"))
			Expect(result.ImageDigest).To(Equal("sha256:abcdef123456789"))
		})

		It("should apply TLS settings to all operations", func() {
			_, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).NotTo(HaveOccurred())

			// Verify TLS settings are applied (minimal verification)
			commands := mockRunner.GetExecutedCommands()
			allCommands := strings.Join([]string{
				strings.Join(commands[0], " "), // build
				strings.Join(commands[1], " "), // push
				strings.Join(commands[2], " "), // inspect
			}, " ")

			// Should contain TLS disable flags for operations that support it
			Expect(allCommands).To(ContainSubstring("--tls-verify=false"))
		})
	})

	Context("when build operation fails", func() {
		BeforeEach(func() {
			// Set default error for any command (simulates build failure)
			mockRunner.DefaultError = &exec.CommandError{ExitCode: 1, Message: "build operation failed"}
		})

		It("should return build error and stop execution", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("build"))
			Expect(result).To(BeNil())
		})
	})

	Context("when push operation fails", func() {
		BeforeEach(func() {
			// Build succeeds, push fails
			mockRunner.SetError(
				"buildah",
				&exec.CommandError{ExitCode: 1, Message: "push failed"},
				"push",
				"quay.io/test/image:latest",
			)
		})

		It("should return push error after successful build", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("push"))
			Expect(result).To(BeNil())
		})
	})

	Context("when digest retrieval fails", func() {
		BeforeEach(func() {
			// Build and push succeed, but digest retrieval fails
			mockRunner.SetError("skopeo",
				&exec.CommandError{ExitCode: 1, Message: "digest retrieval failed"},
				"inspect", "docker://quay.io/test/image:latest")
		})

		It("should complete successfully but return empty digest", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			// The overall operation should succeed
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ImageURL).To(Equal("quay.io/test/image:latest"))

			// But digest should be empty due to retrieval failure
			Expect(result.ImageDigest).To(BeEmpty())
		})
	})

	Context("when digest retrieval returns invalid data", func() {
		BeforeEach(func() {
			// Mock invalid JSON response
			mockRunner.SetOutput(
				"skopeo",
				[]byte("invalid json"),
				"inspect",
				"docker://quay.io/test/image:latest",
			)
		})

		It("should handle invalid digest data gracefully", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			// Should still complete successfully
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ImageURL).To(Equal("quay.io/test/image:latest"))

			// Digest should be empty due to invalid data
			Expect(result.ImageDigest).To(BeEmpty())
		})
	})

	Context("when digest is retrieved successfully", func() {
		BeforeEach(func() {
			// Mock successful digest with different format
			digestResponse := map[string]interface{}{
				"Digest": "sha256:1234567890abcdef",
			}
			digestJSON, _ := json.Marshal(digestResponse)
			mockRunner.SetOutput(
				"skopeo",
				digestJSON,
				"inspect",
				"docker://quay.io/test/image:latest",
			)
		})

		It("should return the correct digest", func() {
			result, err := BuildAndPush(ctx, logger, config, mockRunner)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ImageDigest).To(Equal("sha256:1234567890abcdef"))
		})
	})
})
