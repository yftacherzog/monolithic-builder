package image

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildahBuildCommand", func() {
	Context("when building basic commands", func() {
		It("should generate correct basic build command", func() {
			config := &BuildConfig{
				ImageURL:   "quay.io/test/image:tag",
				Dockerfile: "./Dockerfile",
				TLSVerify:  true,
				BuildArgs:  []string{},
			}

			result := BuildahBuildCommand(config)

			Expect(result).To(Equal([]string{
				"build",
				"--file", "./Dockerfile",
				"--tag", "quay.io/test/image:tag",
				".",
			}))
		})

		It("should include build arguments when provided", func() {
			config := &BuildConfig{
				ImageURL:   "quay.io/test/image:tag",
				Dockerfile: "./Dockerfile",
				TLSVerify:  true,
				BuildArgs:  []string{"GO_VERSION=1.21", "DEBUG=true"},
			}

			result := BuildahBuildCommand(config)

			Expect(result).To(Equal([]string{
				"build",
				"--file", "./Dockerfile",
				"--tag", "quay.io/test/image:tag",
				"--build-arg", "GO_VERSION=1.21",
				"--build-arg", "DEBUG=true",
				".",
			}))
		})

		It("should disable TLS verification when configured", func() {
			config := &BuildConfig{
				ImageURL:   "quay.io/test/image:tag",
				Dockerfile: "./Dockerfile",
				TLSVerify:  false,
				BuildArgs:  []string{"KEY=value"},
			}

			result := BuildahBuildCommand(config)

			Expect(result).To(ContainElements(
				"build",
				"--file", "./Dockerfile",
				"--tag", "quay.io/test/image:tag",
				"--tls-verify=false",
				"--build-arg", "KEY=value",
				".",
			))
		})

		It("should include commit SHA as label when provided", func() {
			config := &BuildConfig{
				ImageURL:   "quay.io/test/image:tag",
				Dockerfile: "./Dockerfile",
				TLSVerify:  true,
				CommitSHA:  "abc123def456",
				BuildArgs:  []string{},
			}

			result := BuildahBuildCommand(config)

			Expect(result).To(ContainElements(
				"build",
				"--file", "./Dockerfile",
				"--tag", "quay.io/test/image:tag",
				"--label", "io.konflux.commit=abc123def456",
				".",
			))
		})
	})

	Context("when configuring hermetic builds", func() {
		It("should add network isolation and volume mounts for hermetic builds", func() {
			config := &BuildConfig{
				ImageURL:      "quay.io/test/image:tag",
				Dockerfile:    "./Dockerfile",
				TLSVerify:     true,
				Hermetic:      true,
				PrefetchInput: "input.json",
				PrefetchPath:  "/workspace/cachi2",
				BuildArgs:     []string{},
			}

			result := BuildahBuildCommand(config)

			Expect(result).To(ContainElement("--network=none"))
			Expect(result).To(ContainElement("--volume"))
		})
	})

	Context("when handling expiration labels", func() {
		It("should add expiration label when ImageExpiresAfter is set", func() {
			config := &BuildConfig{
				ImageURL:          "quay.io/test/image:tag",
				Dockerfile:        "./Dockerfile",
				TLSVerify:         true,
				ImageExpiresAfter: "24h",
				BuildArgs:         []string{},
			}

			result := BuildahBuildCommand(config)

			Expect(result).To(ContainElement("--label"))
			// Find the expiration label
			var expirationLabel string
			for i, arg := range result {
				if arg == "--label" && i+1 < len(result) &&
					strings.HasPrefix(result[i+1], "quay.expires-after=") {
					expirationLabel = result[i+1]
					break
				}
			}
			Expect(expirationLabel).To(HavePrefix("quay.expires-after="))
		})
	})
})

var _ = Describe("UnshareCommand", func() {
	It("should wrap buildah command with proper unshare arguments", func() {
		buildahArgs := []string{"build", "--tag", "test:tag", "."}
		context := "/workspace/source"

		result := UnshareCommand(buildahArgs, context)

		Expect(result).To(Equal([]string{
			"unshare", "-Uf", "--keep-caps", "-r",
			"--map-users", "1,1,65536",
			"--map-groups", "1,1,65536",
			"-w", "/workspace/source",
			"--mount", "--", "sh", "-c",
			`"buildah" "build" "--tag" "test:tag" "."`,
		}))
	})

	It("should properly quote complex buildah arguments", func() {
		buildahArgs := []string{
			"build", "--build-arg", "KEY=value with spaces", "--tag", "test:tag", ".",
		}
		context := "/workspace/source"

		result := UnshareCommand(buildahArgs, context)

		Expect(result).To(HaveLen(15)) // Updated to match actual length
		Expect(result[0]).To(Equal("unshare"))
		Expect(result[len(result)-1]).To(ContainSubstring("KEY=value with spaces"))
	})
})

var _ = Describe("BuildahPushCommand", func() {
	Context("when TLS verification is enabled", func() {
		It("should generate push command without TLS flags", func() {
			config := &BuildConfig{
				ImageURL:  "quay.io/test/image:tag",
				TLSVerify: true,
			}

			result := BuildahPushCommand(config)

			Expect(result).To(Equal([]string{"push", "quay.io/test/image:tag"}))
		})
	})

	Context("when TLS verification is disabled", func() {
		It("should generate push command with TLS verification disabled", func() {
			config := &BuildConfig{
				ImageURL:  "quay.io/test/image:tag",
				TLSVerify: false,
			}

			result := BuildahPushCommand(config)

			Expect(result).To(Equal([]string{
				"push", "--tls-verify=false", "quay.io/test/image:tag"}))
		})
	})
})

var _ = Describe("SkopeoInspectCommand", func() {
	Context("when TLS verification is enabled", func() {
		It("should generate inspect command with docker:// prefix", func() {
			result := SkopeoInspectCommand("quay.io/test/image:tag", true)

			Expect(result).To(Equal([]string{
				"inspect",
				"docker://quay.io/test/image:tag",
			}))
		})
	})

	Context("when TLS verification is disabled", func() {
		It("should generate inspect command with TLS verification disabled", func() {
			result := SkopeoInspectCommand("quay.io/test/image:tag", false)

			Expect(result).To(Equal([]string{
				"inspect",
				"--tls-verify=false",
				"docker://quay.io/test/image:tag",
			}))
		})
	})

	Context("when checking image existence", func() {
		It("should generate exists command with raw flag", func() {
			result := SkopeoExistsCommand("quay.io/test/image:tag", true)

			Expect(result).To(Equal([]string{
				"inspect",
				"--raw",
				"docker://quay.io/test/image:tag",
			}))
		})

		It("should generate exists command with TLS disabled", func() {
			result := SkopeoExistsCommand("quay.io/test/image:tag", false)

			Expect(result).To(Equal([]string{
				"inspect",
				"--raw",
				"--tls-verify=false",
				"docker://quay.io/test/image:tag",
			}))
		})
	})
})
