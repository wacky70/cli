package application_test

import (
	"github.com/cloudfoundry/cli/cf/api/apifakes"
	"github.com/cloudfoundry/cli/cf/api/applications/applicationsfakes"
	"github.com/cloudfoundry/cli/cf/api/authentication/authenticationfakes"
	"github.com/cloudfoundry/cli/cf/api/copyapplicationsource/copyapplicationsourcefakes"
	"github.com/cloudfoundry/cli/cf/api/organizations/organizationsfakes"
	"github.com/cloudfoundry/cli/cf/commands/application/applicationfakes"
	"github.com/cloudfoundry/cli/cf/models"
	testcmd "github.com/cloudfoundry/cli/testhelpers/commands"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testreq "github.com/cloudfoundry/cli/testhelpers/requirements"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"

	"github.com/cloudfoundry/cli/cf/commandregistry"
	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	"github.com/cloudfoundry/cli/cf/errors"

	. "github.com/cloudfoundry/cli/testhelpers/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CopySource", func() {

	var (
		ui                  *testterm.FakeUI
		config              coreconfig.Repository
		requirementsFactory *testreq.FakeReqFactory
		authRepo            *authenticationfakes.FakeAuthenticationRepository
		appRepo             *applicationsfakes.FakeApplicationRepository
		copyAppSourceRepo   *copyapplicationsourcefakes.FakeCopyApplicationSourceRepository
		spaceRepo           *apifakes.FakeSpaceRepository
		orgRepo             *organizationsfakes.FakeOrganizationRepository
		appRestarter        *applicationfakes.FakeApplicationRestarter
		OriginalCommand     commandregistry.Command
		deps                commandregistry.Dependency
	)

	updateCommandDependency := func(pluginCall bool) {
		deps.UI = ui
		deps.RepoLocator = deps.RepoLocator.SetAuthenticationRepository(authRepo)
		deps.RepoLocator = deps.RepoLocator.SetApplicationRepository(appRepo)
		deps.RepoLocator = deps.RepoLocator.SetCopyApplicationSourceRepository(copyAppSourceRepo)
		deps.RepoLocator = deps.RepoLocator.SetSpaceRepository(spaceRepo)
		deps.RepoLocator = deps.RepoLocator.SetOrganizationRepository(orgRepo)
		deps.Config = config

		//inject fake 'command dependency' into registry
		commandregistry.Register(appRestarter)

		commandregistry.Commands.SetCommand(commandregistry.Commands.FindCommand("copy-source").SetDependency(deps, pluginCall))
	}

	BeforeEach(func() {
		ui = &testterm.FakeUI{}
		requirementsFactory = &testreq.FakeReqFactory{LoginSuccess: true, TargetedSpaceSuccess: true}
		authRepo = new(authenticationfakes.FakeAuthenticationRepository)
		appRepo = new(applicationsfakes.FakeApplicationRepository)
		copyAppSourceRepo = new(copyapplicationsourcefakes.FakeCopyApplicationSourceRepository)
		spaceRepo = new(apifakes.FakeSpaceRepository)
		orgRepo = new(organizationsfakes.FakeOrganizationRepository)
		config = testconfig.NewRepositoryWithDefaults()

		//save original command and restore later
		OriginalCommand = commandregistry.Commands.FindCommand("restart")

		appRestarter = new(applicationfakes.FakeApplicationRestarter)
		//setup fakes to correctly interact with commandregistry
		appRestarter.SetDependencyStub = func(_ commandregistry.Dependency, _ bool) commandregistry.Command {
			return appRestarter
		}
		appRestarter.MetaDataReturns(commandregistry.CommandMetadata{Name: "restart"})
	})

	AfterEach(func() {
		commandregistry.Register(OriginalCommand)
	})

	runCommand := func(args ...string) bool {
		return testcmd.RunCLICommand("copy-source", args, requirementsFactory, updateCommandDependency, false, ui)
	}

	Describe("requirement failures", func() {
		It("when not logged in", func() {
			requirementsFactory.LoginSuccess = false
			Expect(runCommand("source-app", "target-app")).ToNot(HavePassedRequirements())
		})

		It("when a space is not targeted", func() {
			requirementsFactory.TargetedSpaceSuccess = false
			Expect(runCommand("source-app", "target-app")).ToNot(HavePassedRequirements())
		})

		It("when provided too many args", func() {
			Expect(runCommand("source-app", "target-app", "too-much", "app-name")).ToNot(HavePassedRequirements())
		})
	})

	Describe("Passing requirements", func() {
		BeforeEach(func() {
			requirementsFactory.LoginSuccess = true
			requirementsFactory.TargetedSpaceSuccess = true
		})

		Context("refreshing the auth token", func() {
			It("makes a call for the app token", func() {
				runCommand("source-app", "target-app")
				Expect(authRepo.RefreshAuthTokenCallCount()).To(Equal(1))
			})

			Context("when refreshing the auth token fails", func() {
				BeforeEach(func() {
					authRepo.RefreshAuthTokenReturns("", errors.New("I accidentally the UAA"))
				})

				It("it displays an error", func() {
					runCommand("source-app", "target-app")
					Expect(ui.Outputs).To(ContainSubstrings(
						[]string{"FAILED"},
						[]string{"accidentally the UAA"},
					))
				})
			})

			Describe("when retrieving the app token succeeds", func() {
				var (
					sourceApp, targetApp models.Application
				)

				BeforeEach(func() {
					sourceApp = models.Application{
						ApplicationFields: models.ApplicationFields{
							Name: "source-app",
							GUID: "source-app-guid",
						},
					}
					appRepo.ReadReturns(sourceApp, nil)

					targetApp = models.Application{
						ApplicationFields: models.ApplicationFields{
							Name: "target-app",
							GUID: "target-app-guid",
						},
					}
					appRepo.ReadFromSpaceReturns(targetApp, nil)
				})

				Describe("when no parameters are passed", func() {
					It("obtains both the source and target application from the same space", func() {
						runCommand("source-app", "target-app")

						targetAppName, spaceGUID := appRepo.ReadFromSpaceArgsForCall(0)
						Expect(targetAppName).To(Equal("target-app"))
						Expect(spaceGUID).To(Equal("my-space-guid"))

						Expect(appRepo.ReadArgsForCall(0)).To(Equal("source-app"))

						sourceAppGUID, targetAppGUID := copyAppSourceRepo.CopyApplicationArgsForCall(0)
						Expect(sourceAppGUID).To(Equal("source-app-guid"))
						Expect(targetAppGUID).To(Equal("target-app-guid"))

						appArg, orgName, spaceName := appRestarter.ApplicationRestartArgsForCall(0)
						Expect(appArg).To(Equal(targetApp))
						Expect(orgName).To(Equal(config.OrganizationFields().Name))
						Expect(spaceName).To(Equal(config.SpaceFields().Name))

						Expect(ui.Outputs).To(ContainSubstrings(
							[]string{"Copying source from app", "source-app", "to target app", "target-app", "in org my-org / space my-space as my-user..."},
							[]string{"Note: this may take some time"},
							[]string{"OK"},
						))
					})

					Context("Failures", func() {
						It("if we cannot obtain the source application", func() {
							appRepo.ReadReturns(models.Application{}, errors.New("could not find source app"))
							runCommand("source-app", "target-app")

							Expect(ui.Outputs).To(ContainSubstrings(
								[]string{"FAILED"},
								[]string{"could not find source app"},
							))
						})

						It("fails if we cannot obtain the target application", func() {
							appRepo.ReadFromSpaceReturns(models.Application{}, errors.New("could not find target app"))
							runCommand("source-app", "target-app")

							Expect(ui.Outputs).To(ContainSubstrings(
								[]string{"FAILED"},
								[]string{"could not find target app"},
							))
						})
					})
				})

				Describe("when a space is provided, but not an org", func() {
					It("send the correct target appplication for the current org and target space", func() {
						space := models.Space{}
						space.Name = "space-name"
						space.GUID = "model-space-guid"
						spaceRepo.FindByNameReturns(space, nil)

						runCommand("-s", "space-name", "source-app", "target-app")

						targetAppName, spaceGUID := appRepo.ReadFromSpaceArgsForCall(0)
						Expect(targetAppName).To(Equal("target-app"))
						Expect(spaceGUID).To(Equal("model-space-guid"))

						Expect(ui.Outputs).To(ContainSubstrings(
							[]string{"Copying source from app", "source-app", "to target app", "target-app", "in org my-org / space space-name as my-user..."},
							[]string{"Note: this may take some time"},
							[]string{"OK"},
						))
					})

					Context("Failures", func() {
						It("when we cannot find the provided space", func() {
							spaceRepo.FindByNameReturns(models.Space{}, errors.New("Error finding space by name."))

							runCommand("-s", "space-name", "source-app", "target-app")
							Expect(ui.Outputs).To(ContainSubstrings(
								[]string{"FAILED"},
								[]string{"Error finding space by name."},
							))
						})
					})
				})

				Describe("when an org and space name are passed as parameters", func() {
					It("send the correct target application for the space and org", func() {
						orgRepo.FindByNameReturns(models.Organization{
							Spaces: []models.SpaceFields{
								{
									Name: "space-name",
									GUID: "space-guid",
								},
							},
						}, nil)

						runCommand("-o", "org-name", "-s", "space-name", "source-app", "target-app")

						targetAppName, spaceGUID := appRepo.ReadFromSpaceArgsForCall(0)
						Expect(targetAppName).To(Equal("target-app"))
						Expect(spaceGUID).To(Equal("space-guid"))

						sourceAppGUID, targetAppGUID := copyAppSourceRepo.CopyApplicationArgsForCall(0)
						Expect(sourceAppGUID).To(Equal("source-app-guid"))
						Expect(targetAppGUID).To(Equal("target-app-guid"))

						appArg, orgName, spaceName := appRestarter.ApplicationRestartArgsForCall(0)
						Expect(appArg).To(Equal(targetApp))
						Expect(orgName).To(Equal("org-name"))
						Expect(spaceName).To(Equal("space-name"))

						Expect(ui.Outputs).To(ContainSubstrings(
							[]string{"Copying source from app source-app to target app target-app in org org-name / space space-name as my-user..."},
							[]string{"Note: this may take some time"},
							[]string{"OK"},
						))
					})

					Context("failures", func() {
						It("cannot just accept an organization and no space", func() {
							runCommand("-o", "org-name", "source-app", "target-app")

							Expect(ui.Outputs).To(ContainSubstrings(
								[]string{"FAILED"},
								[]string{"Please provide the space within the organization containing the target application"},
							))
						})

						It("when we cannot find the provided org", func() {
							orgRepo.FindByNameReturns(models.Organization{}, errors.New("Could not find org"))
							runCommand("-o", "org-name", "-s", "space-name", "source-app", "target-app")

							Expect(ui.Outputs).To(ContainSubstrings(
								[]string{"FAILED"},
								[]string{"Could not find org"},
							))
						})

						It("when the org does not contain the space name provide", func() {
							orgRepo.FindByNameReturns(models.Organization{}, nil)
							runCommand("-o", "org-name", "-s", "space-name", "source-app", "target-app")

							Expect(ui.Outputs).To(ContainSubstrings(
								[]string{"FAILED"},
								[]string{"Could not find space space-name in organization org-name"},
							))
						})

						It("when the targeted app does not exist in the targeted org and space", func() {
							orgRepo.FindByNameReturns(models.Organization{
								Spaces: []models.SpaceFields{
									{
										Name: "space-name",
										GUID: "space-guid",
									},
								},
							}, nil)

							appRepo.ReadFromSpaceReturns(models.Application{}, errors.New("could not find app"))
							runCommand("-o", "org-name", "-s", "space-name", "source-app", "target-app")

							Expect(ui.Outputs).To(ContainSubstrings(
								[]string{"FAILED"},
								[]string{"could not find app"},
							))
						})
					})
				})

				Describe("when the --no-restart flag is passed", func() {
					It("does not restart the target application", func() {
						runCommand("--no-restart", "source-app", "target-app")
						Expect(appRestarter.ApplicationRestartCallCount()).To(Equal(0))
					})
				})
			})
		})
	})
})
