package templatescompiler_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"

	fakeblobs "github.com/cloudfoundry/bosh-agent/blobstore/fakes"
	fakecmd "github.com/cloudfoundry/bosh-agent/platform/commands/fakes"
	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
	fakebmtemp "github.com/cloudfoundry/bosh-micro-cli/templatescompiler/fakes"

	bmrel "github.com/cloudfoundry/bosh-micro-cli/release"
	. "github.com/cloudfoundry/bosh-micro-cli/templatescompiler"
)

var _ = Describe("TemplatesCompiler", func() {
	var (
		templatesCompiler    TemplatesCompiler
		jobRenderer          *fakebmtemp.FakeJobRenderer
		compressor           *fakecmd.FakeCompressor
		blobstore            *fakeblobs.FakeBlobstore
		templatesRepo        *fakebmtemp.FakeTemplatesRepo
		fs                   *fakesys.FakeFileSystem
		compileDir           string
		jobs                 []bmrel.Job
		deploymentProperties map[string]interface{}
		logger               boshlog.Logger
	)

	BeforeEach(func() {
		jobRenderer = fakebmtemp.NewFakeJobRenderer()
		compressor = fakecmd.NewFakeCompressor()
		compressor.CompressFilesInDirTarballPath = "fake-tarball-path"

		blobstore = fakeblobs.NewFakeBlobstore()
		fs = fakesys.NewFakeFileSystem()

		templatesRepo = fakebmtemp.NewFakeTemplatesRepo()

		deploymentProperties = map[string]interface{}{
			"fake-property-key": "fake-property-value",
		}

		logger = boshlog.NewLogger(boshlog.LevelNone)

		templatesCompiler = NewTemplatesCompiler(
			jobRenderer,
			compressor,
			blobstore,
			templatesRepo,
			fs,
			logger,
		)

		var err error
		compileDir, err = fs.TempDir("bosh-micro-cli-tests")
		Expect(err).ToNot(HaveOccurred())
		fs.TempDirDir = compileDir
	})

	Context("with a job", func() {
		BeforeEach(func() {
			jobs = []bmrel.Job{
				bmrel.Job{
					Name:          "fake-job-1",
					ExtractedPath: "fake-extracted-path",
					Templates: map[string]string{
						"cpi.erb": "/bin/cpi",
					},
				},
			}

			blobstore.CreateBlobID = "fake-blob-id"
			blobstore.CreateFingerprint = "fake-sha1"
			record := TemplateRecord{
				BlobID:   "fake-blob-id",
				BlobSHA1: "fake-sha1",
			}
			templatesRepo.SetSaveBehavior(jobs[0], record, nil)
		})

		It("renders job templates", func() {
			fs.TempDirDir = "/fake-temp-dir"
			err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
			Expect(err).ToNot(HaveOccurred())
			Expect(jobRenderer.RenderInputs).To(ContainElement(
				fakebmtemp.RenderInput{
					SourcePath:      "fake-extracted-path",
					DestinationPath: "/fake-temp-dir",
					Job: bmrel.Job{
						Name:          "fake-job-1",
						Fingerprint:   "",
						SHA1:          "",
						ExtractedPath: "fake-extracted-path",
						Templates: map[string]string{
							"cpi.erb": "/bin/cpi",
						},
						PackageNames: nil,
						Packages:     nil,
						Properties:   nil,
					},
					Properties: map[string]interface{}{
						"fake-property-key": "fake-property-value",
					},
					DeploymentName: "fake-deployment-name",
				}),
			)
		})

		It("cleans the temp folder to hold the compile result for job", func() {
			err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
			Expect(err).ToNot(HaveOccurred())
			Expect(fs.FileExists(compileDir)).To(BeFalse())
		})

		It("generates templates archive", func() {
			err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
			Expect(err).ToNot(HaveOccurred())
			Expect(compressor.CompressFilesInDirDir).To(Equal(compileDir))
			Expect(compressor.CleanUpTarballPath).To(Equal("fake-tarball-path"))
		})

		It("saves archive in blobstore", func() {
			err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
			Expect(err).ToNot(HaveOccurred())
			Expect(blobstore.CreateFileNames).To(Equal([]string{"fake-tarball-path"}))
		})

		It("stores the compiled package blobID and fingerprint into the compile package repo", func() {
			err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
			Expect(err).ToNot(HaveOccurred())

			record := TemplateRecord{
				BlobID:   "fake-blob-id",
				BlobSHA1: "fake-sha1",
			}

			Expect(templatesRepo.SaveInputs).To(ContainElement(
				fakebmtemp.SaveInput{Job: jobs[0], Record: record},
			))
		})

		Context("when creating compilation directory fails", func() {
			BeforeEach(func() {
				fs.TempDirError = errors.New("fake-tempdir-error")
			})

			It("returns an error", func() {
				err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-tempdir-error"))
			})
		})

		Context("when rendering fails", func() {
			BeforeEach(func() {
				jobRenderer.SetRenderBehavior(
					"fake-extracted-path",
					errors.New("fake-render-error"),
				)
			})

			It("returns an error", func() {
				err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-render-error"))
			})
		})

		Context("when generating templates archive fails", func() {
			BeforeEach(func() {
				compressor.CompressFilesInDirErr = errors.New("fake-compress-error")
			})

			It("returns an error", func() {
				err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-compress-error"))
			})
		})

		Context("when saving to blobstore fails", func() {
			BeforeEach(func() {
				blobstore.CreateErr = errors.New("fake-blobstore-error")
			})

			It("returns an error", func() {
				err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-blobstore-error"))
			})
		})

		Context("when saving to templates repo fails", func() {
			BeforeEach(func() {
				record := TemplateRecord{
					BlobID:   "fake-blob-id",
					BlobSHA1: "fake-sha1",
				}

				err := errors.New("fake-template-error")
				templatesRepo.SetSaveBehavior(jobs[0], record, err)
			})

			It("returns an error", func() {
				err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-template-error"))
			})
		})

		Context("when one of the job fails to compile", func() {
			BeforeEach(func() {
				jobs = []bmrel.Job{
					bmrel.Job{
						Name:          "fake-job-1",
						ExtractedPath: "fake-extracted-path-1",
						Templates: map[string]string{
							"cpi.erb": "/bin/cpi",
						},
					},
					bmrel.Job{
						Name:          "fake-job-2",
						ExtractedPath: "fake-extracted-path-2",
						Templates: map[string]string{
							"cpi.erb": "/bin/cpi",
						},
					},
					bmrel.Job{
						Name:          "fake-job-3",
						ExtractedPath: "fake-extracted-path-3",
						Templates: map[string]string{
							"cpi.erb": "/bin/cpi",
						},
					},
				}

				jobRenderer.SetRenderBehavior(
					"fake-extracted-path-1",
					nil,
				)

				jobRenderer.SetRenderBehavior(
					"fake-extracted-path-2",
					errors.New("fake-render-2-error"),
				)

				record := TemplateRecord{
					BlobID:   "fake-blob-id",
					BlobSHA1: "fake-sha1",
				}
				templatesRepo.SetSaveBehavior(jobs[0], record, nil)
				templatesRepo.SetSaveBehavior(jobs[1], record, nil)
			})

			It("returns an error", func() {
				err := templatesCompiler.Compile(jobs, "fake-deployment-name", deploymentProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-render-2-error"))
			})
		})
	})
})
