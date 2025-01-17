package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testOffline(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually
		pack       occam.Pack
		docker     occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when offline", func() {
		var (
			image     occam.Image
			container occam.Container
			name      string
			source    string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("creates a working OCI image with specified version of php on PATH", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					offlinePhpDistBuildpack,
					buildPlanBuildpack,
				).
				WithEnv(map[string]string{"BP_PHP_VERSION": "8.0.*"}).
				WithNetwork("none").
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred(), logs.String())

			Expect(logs.String()).To(ContainSubstring(buildpackInfo.Buildpack.Name))
			Expect(logs.String()).To(MatchRegexp(`PHP 8\.0\.\d+`))
			Expect(logs.String()).NotTo(ContainSubstring("Downloading"))

			container, err = docker.Container.Run.WithCommand("php -i && sleep infinity").Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(container.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(
				And(
					MatchRegexp(`PHP Version => 8\.0\.\d+`),
					ContainSubstring(
						fmt.Sprintf("PHP_HOME => /layers/%s/php", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
					),
					ContainSubstring(
						fmt.Sprintf("MIBDIRS => /layers/%s/php/mibs", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
					),
					MatchRegexp(
						fmt.Sprintf(`PHP_EXTENSION_DIR => /layers/%s/php/lib/php/extensions/no-debug-non-zts-\d+`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
					),
				),
			)
		})
	})
}
