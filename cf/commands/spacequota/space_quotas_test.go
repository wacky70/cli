package spacequota_test

import (
	"github.com/cloudfoundry/cli/cf/api/spacequotas/spacequotasfakes"
	"github.com/cloudfoundry/cli/cf/commandregistry"
	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	"github.com/cloudfoundry/cli/cf/errors"
	"github.com/cloudfoundry/cli/cf/models"
	"github.com/cloudfoundry/cli/flags"
	testcmd "github.com/cloudfoundry/cli/testhelpers/commands"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testreq "github.com/cloudfoundry/cli/testhelpers/requirements"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"

	"github.com/cloudfoundry/cli/cf/commands/spacequota"
	. "github.com/cloudfoundry/cli/testhelpers/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("quotas command", func() {
	var (
		ui                  *testterm.FakeUI
		quotaRepo           *spacequotasfakes.FakeSpaceQuotaRepository
		configRepo          coreconfig.Repository
		requirementsFactory *testreq.FakeReqFactory
		deps                commandregistry.Dependency
	)

	updateCommandDependency := func(pluginCall bool) {
		deps.UI = ui
		deps.RepoLocator = deps.RepoLocator.SetSpaceQuotaRepository(quotaRepo)
		deps.Config = configRepo
		commandregistry.Commands.SetCommand(commandregistry.Commands.FindCommand("space-quotas").SetDependency(deps, pluginCall))
	}

	BeforeEach(func() {
		ui = &testterm.FakeUI{}
		quotaRepo = new(spacequotasfakes.FakeSpaceQuotaRepository)
		requirementsFactory = &testreq.FakeReqFactory{LoginSuccess: true}
		configRepo = testconfig.NewRepositoryWithDefaults()
	})

	runCommand := func(args ...string) bool {
		return testcmd.RunCLICommand("space-quotas", args, requirementsFactory, updateCommandDependency, false, ui)
	}

	Describe("requirements", func() {
		It("requires the user to be logged in", func() {
			requirementsFactory.LoginSuccess = false
			Expect(runCommand()).ToNot(HavePassedRequirements())
		})

		It("requires the user to target an org", func() {
			requirementsFactory.TargetedOrgSuccess = false
			Expect(runCommand()).ToNot(HavePassedRequirements())
		})

		Context("when arguments are provided", func() {
			var cmd commandregistry.Command
			var flagContext flags.FlagContext

			BeforeEach(func() {
				cmd = &spacequota.ListSpaceQuotas{}
				cmd.SetDependency(deps, false)
				flagContext = flags.NewFlagContext(cmd.MetaData().Flags)
			})

			It("should fail with usage", func() {
				flagContext.Parse("blahblah")

				reqs := cmd.Requirements(requirementsFactory, flagContext)

				err := testcmd.RunRequirements(reqs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Incorrect Usage"))
				Expect(err.Error()).To(ContainSubstring("No argument required"))
			})
		})
	})

	Context("when requirements have been met", func() {
		JustBeforeEach(func() {
			requirementsFactory.LoginSuccess = true
			requirementsFactory.TargetedOrgSuccess = true
			runCommand()
		})

		Context("when quotas exist", func() {
			BeforeEach(func() {
				quotaRepo.FindByOrgReturns([]models.SpaceQuota{
					{
						Name:                    "quota-name",
						MemoryLimit:             1024,
						InstanceMemoryLimit:     512,
						RoutesLimit:             111,
						ServicesLimit:           222,
						NonBasicServicesAllowed: true,
						OrgGUID:                 "my-org-guid",
						AppInstanceLimit:        7,
						ReservedRoutePortsLimit: "6",
					},
					{
						Name:                    "quota-non-basic-not-allowed",
						MemoryLimit:             434,
						InstanceMemoryLimit:     -1,
						RoutesLimit:             1,
						ServicesLimit:           2,
						NonBasicServicesAllowed: false,
						OrgGUID:                 "my-org-guid",
						AppInstanceLimit:        1,
						ReservedRoutePortsLimit: "3",
					},
					{
						Name:                    "quota-app-instances",
						MemoryLimit:             434,
						InstanceMemoryLimit:     512,
						RoutesLimit:             1,
						ServicesLimit:           2,
						NonBasicServicesAllowed: false,
						OrgGUID:                 "my-org-guid",
						AppInstanceLimit:        -1,
						ReservedRoutePortsLimit: "0",
					},
				}, nil)
			})

			It("lists quotas", func() {
				Expect(quotaRepo.FindByOrgArgsForCall(0)).To(Equal("my-org-guid"))
				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"Getting space quotas as", "my-user"},
					[]string{"OK"},
					[]string{"name", "total memory", "instance memory", "routes", "service instances", "paid plans", "app instances"},
					[]string{"quota-name", "1G", "512M", "111", "222", "allowed", "7", "6"},
					[]string{"quota-non-basic-not-allowed", "434M", "unlimited", "1", "2", "disallowed", "1", "3"},
					[]string{"quota-app-instances", "434M", "512M", "1", "2", "disallowed", "unlimited", "0"},
				))
			})

			Context("when services are unlimited", func() {
				BeforeEach(func() {
					quotaRepo.FindByOrgReturns([]models.SpaceQuota{
						{
							Name:                    "quota-non-basic-not-allowed",
							MemoryLimit:             434,
							InstanceMemoryLimit:     57,
							RoutesLimit:             1,
							ServicesLimit:           -1,
							NonBasicServicesAllowed: false,
							OrgGUID:                 "my-org-guid",
						},
					}, nil)
				})

				It("replaces -1 with unlimited", func() {
					Expect(quotaRepo.FindByOrgArgsForCall(0)).To(Equal("my-org-guid"))
					Expect(ui.Outputs).To(ContainSubstrings(

						[]string{"quota-non-basic-not-allowed", "434M", "57M ", "1", "unlimited", "disallowed"},
					))
				})

			})

			Context("when reserved route ports are unlimited", func() {
				BeforeEach(func() {
					quotaRepo.FindByOrgReturns([]models.SpaceQuota{
						{
							Name:                    "quota-non-basic-not-allowed",
							MemoryLimit:             434,
							InstanceMemoryLimit:     57,
							RoutesLimit:             1,
							ServicesLimit:           6,
							NonBasicServicesAllowed: false,
							OrgGUID:                 "my-org-guid",
							ReservedRoutePortsLimit: "-1",
						},
					}, nil)
				})

				It("replaces -1 with unlimited", func() {
					Expect(quotaRepo.FindByOrgArgsForCall(0)).To(Equal("my-org-guid"))
					Expect(ui.Outputs).To(ContainSubstrings(

						[]string{"quota-non-basic-not-allowed", "434M", "57M ", "1", "6", "disallowed", "unlimited"},
					))
				})
			})

			Context("when app instances are not provided", func() {
				BeforeEach(func() {
					quotaRepo.FindByOrgReturns([]models.SpaceQuota{
						{
							Name:                    "quota-non-basic-not-allowed",
							MemoryLimit:             434,
							InstanceMemoryLimit:     57,
							RoutesLimit:             1,
							ServicesLimit:           512,
							NonBasicServicesAllowed: false,
							OrgGUID:                 "my-org-guid",
							AppInstanceLimit:        -1,
						},
					}, nil)
				})

				It("should not contain app instance limit column", func() {
					Expect(quotaRepo.FindByOrgArgsForCall(0)).To(Equal("my-org-guid"))
					Expect(ui.Outputs).To(ContainSubstrings(
						[]string{"app instances"},
						[]string{"unlimited"},
					))
				})
			})
		})

		Context("when an error occurs fetching quotas", func() {
			BeforeEach(func() {
				quotaRepo.FindByOrgReturns([]models.SpaceQuota{}, errors.New("I haz a borken!"))
			})

			It("prints an error", func() {
				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"Getting space quotas as", "my-user"},
					[]string{"FAILED"},
				))
			})
		})
	})

})
