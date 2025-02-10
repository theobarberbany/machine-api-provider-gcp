package util_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	computeservice "github.com/openshift/machine-api-provider-gcp/pkg/cloud/gcp/actuators/services/compute"
	"github.com/openshift/machine-api-provider-gcp/pkg/cloud/gcp/actuators/util"
)

var _ = Describe("IsUEFICompatible", func() {
	var (
		computeService computeservice.GCPComputeService
		providerSpec   *machinev1.GCPMachineProviderSpec

		compatible bool
		err        error
	)

	BeforeEach(func() {
		_, computeService = computeservice.NewComputeServiceMock()
		providerSpec = &machinev1.GCPMachineProviderSpec{}
	})

	type standardImageInput struct {
		boot                 bool
		image                string
		projectID            string
		zone                 string
		expectedErrSubstring string
		compatible           bool
	}

	var tableFunc func(in standardImageInput) = func(in standardImageInput) {
		providerSpec.Disks = []*machinev1.GCPDisk{
			{
				Boot:  in.boot,
				Image: in.image,
			},
		}

		if in.projectID != "" {
			providerSpec.ProjectID = in.projectID
		}

		if in.zone != "" {
			providerSpec.Zone = in.zone
		}

		compatible, err = util.IsUEFICompatible(computeService, providerSpec)
		if in.expectedErrSubstring != "" {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(in.expectedErrSubstring))
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(compatible).To(Equal(in.compatible))
	}

	DescribeTable("Standard image references in format projects/{project}/global/images/{image}",
		tableFunc,
		Entry("UEFI compatible image", standardImageInput{
			boot:       true,
			image:      "projects/fooproject/global/images/uefi-image",
			compatible: true,
		}),
		Entry("Non-UEFI image", standardImageInput{
			boot:       true,
			image:      "projects/fooproject/global/images/fooimage",
			compatible: false,
		}),
		Entry("Image not found", standardImageInput{
			boot:                 true,
			image:                "projects/errImageNotFound/global/images/fooimage",
			expectedErrSubstring: "unable to retrieve image",
			compatible:           false,
		}),
		Entry("Malformed image reference", standardImageInput{
			boot:                 true,
			image:                "projects/errImageNotFound//asdf/images456/fooimage",
			expectedErrSubstring: "unrecognized image path format",
			compatible:           false,
		}),
	)

	DescribeTable("Simple image names",
		tableFunc,
		Entry("Simple UEFI compatible image", standardImageInput{
			boot:       true,
			image:      "uefi-image",
			projectID:  "simple-project",
			compatible: true,
		}),
		Entry("Simple non-UEFI image", standardImageInput{
			boot:       true,
			image:      "non-uefi",
			projectID:  "simple-project",
			compatible: false,
		}),
		Entry("Simple image not found", standardImageInput{
			boot:                 true,
			image:                "nonexistent",
			projectID:            "errImageNotFound",
			expectedErrSubstring: "unable to retrieve image",
			compatible:           false,
		}),
	)

	DescribeTable("Family image references in format projects/{project}/global/images/family/{imageFamily}",
		tableFunc,
		Entry("UEFI compatible image family", standardImageInput{
			boot:       true,
			image:      "projects/fooproject/global/images/family/uefi-image-family",
			zone:       "us-central1-a",
			compatible: true,
		}),
		Entry("Non-UEFI image family", standardImageInput{
			boot:       true,
			image:      "projects/fooproject/global/images/family/fooimage",
			zone:       "us-central1-a",
			compatible: false,
		}),
		Entry("Image family not found", standardImageInput{
			boot:                 true,
			image:                "projects/errImageNotFound/global/images/family/fooimage",
			zone:                 "us-central1-a",
			expectedErrSubstring: "unable to retrieve image family",
			compatible:           false,
		}),
	)

	DescribeTable("FQDN image URLs",
		tableFunc,
		Entry("UEFI compatible FQDN image", standardImageInput{
			boot:       true,
			image:      "https://www.googleapis.com/compute/v1/projects/fooproject/global/images/uefi-image",
			compatible: true,
		}),
		Entry("Non-UEFI FQDN image", standardImageInput{
			boot:       true,
			image:      "https://www.googleapis.com/compute/v1/projects/fooproject/global/images/fooimage",
			compatible: false,
		}),
		Entry("FQDN URL missing 'projects/' segment", standardImageInput{
			boot:                 true,
			image:                "https://www.googleapis.com/compute/v1/global/images/uefi-image",
			expectedErrSubstring: "does not contain expected 'projects/'",
			compatible:           false,
		}),
		Entry("FQDN URL with incomplete segments", standardImageInput{
			boot:                 true,
			image:                "https://www.googleapis.com/compute/v1/projects/fooproject/global/images",
			expectedErrSubstring: "unexpected image path format",
			compatible:           false,
		}),
		Entry("FQDN URL not following recognized pattern", standardImageInput{
			boot:                 true,
			image:                "https://www.googleapis.com/compute/v1/projects/fooproject/global/foo/uefi-image",
			expectedErrSubstring: "unrecognized image path format",
			compatible:           false,
		}),
	)

	Context("Corner cases", func() {
		JustBeforeEach(func() {
			compatible, err = util.IsUEFICompatible(computeService, providerSpec)
		})
		Context("When no boot disk is found", func() {
			BeforeEach(func() {
				// All disks have Boot set to false.
				providerSpec.Disks = []*machinev1.GCPDisk{
					{
						Boot:  false,
						Image: "projects/fooproject/global/images/uefi-image",
					},
					{
						Boot:  false,
						Image: "projects/fooproject/global/images/fooimage",
					},
				}
			})
			It("should error with a no boot disk message", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no boot disk found"))
			})
		})
		Context("When the boot disk is not the first disk in the list", func() {
			BeforeEach(func() {
				providerSpec.ProjectID = "simple-project"
				// First disk is non-boot; the second disk is boot and UEFI compatible.
				providerSpec.Disks = []*machinev1.GCPDisk{
					{
						Boot:  false,
						Image: "non-uefi-simple",
					},
					{
						Boot:  true,
						Image: "uefi-image",
					},
				}
			})
			It("should process the first boot disk it finds", func() {
				// Since the only boot disk is UEFI compatible, the function should return true.
				Expect(err).ToNot(HaveOccurred())
				Expect(compatible).To(BeTrue())
			})
		})

	})

})
