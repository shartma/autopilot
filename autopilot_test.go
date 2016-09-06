package main_test

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/MerrillCorporation/autopilot"

	 "github.com/cloudfoundry/cli/plugin/pluginfakes"
	 plugin_models "code.cloudfoundry.org/cli/plugin/models"
)

func TestAutopilot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Autopilot Suite")
}

var _ = Describe("Flag Parsing", func() {
	It("parses a complete set of args", func() {
		appName, manifestPath, appPath, options, err := ParseArgs(
			[]string{
				"zero-downtime-push",
				"appname",
				"-f", "manifest-path",
				"-p", "app-path",
				"--keep-existing-app",
			},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(appName).To(Equal("appname"))
		Expect(manifestPath).To(Equal("manifest-path"))
		Expect(appPath).To(Equal("app-path"))
		Expect(options.KeepExisting).To(Equal(true))
	})

	It("adds the unmap-route flag", func() {
		appName, manifestPath, appPath, options, err := ParseArgs(
			[]string{
				"zero-downtime-push",
				"appname",
				"-f", "manifest-path",
				"-p", "app-path",
				"--unmap-routes",
			},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(appName).To(Equal("appname"))
		Expect(manifestPath).To(Equal("manifest-path"))
		Expect(appPath).To(Equal("app-path"))
		Expect(options.UnmapRoute).To(Equal(true))
	})

	It("requires a manifest", func() {
		_, _, _, _, err := ParseArgs(
			[]string{
				"zero-downtime-push",
				"appname",
				"-p", "app-path",
			},
		)
		Expect(err).To(MatchError(ErrNoManifest))
	})
})

var _ = Describe("Option defaults", func() {
	It("properly sets default values for optional options", func() {
		appName, manifestPath, appPath, options, err := ParseArgs(
			[]string{
				"zero-downtime-push",
				"appname",
				"-f", "manifest-path",
				"-p", "app-path",
			},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(appName).To(Equal("appname"))
		Expect(manifestPath).To(Equal("manifest-path"))
		Expect(appPath).To(Equal("app-path"))
		//Defaults:
		Expect(options.KeepExisting).To(Equal(false))
	})
})

var _ = Describe("ApplicationRepo", func() {
	var (
		cliConn    *pluginfakes.FakeCliConnection
		repo       *ApplicationRepo
		route	   Route
		blankRoute Route

	)

	BeforeEach(func() {
		cliConn = &pluginfakes.FakeCliConnection{}
		repo = NewApplicationRepo(cliConn)
		route = Route{Domain: "test-domain.com", Host: []string{"host-app", "host-app-copy"}}
		blankRoute = Route{}
	})

	Describe("RenameApplication", func() {
		It("renames the application", func() {
			err := repo.RenameApplication("old-name", "new-name")
			Expect(err).ToNot(HaveOccurred())

			Expect(cliConn.CliCommandCallCount()).To(Equal(1))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{"rename", "old-name", "new-name"}))
		})

		It("returns an error if one occurs", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("no app"))

			err := repo.RenameApplication("old-name", "new-name")
			Expect(err).To(MatchError("no app"))
		})
	})

	Describe("DoesAppExist", func() {

		It("returns an error if the cli returns an error", func() {
			cliConn.CliCommandWithoutTerminalOutputReturns([]string{}, errors.New("you shall not curl"))
			_, err := repo.DoesAppExist("app-name")

			Expect(err).To(MatchError("you shall not curl"))
		})

		It("returns an error if the cli response is invalid JSON", func() {
			response := []string{
				"}notjson{",
			}

			cliConn.CliCommandWithoutTerminalOutputReturns(response, nil)
			_, err := repo.DoesAppExist("app-name")

			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the cli response doesn't contain total_results", func() {
			response := []string{
				`{"brutal_results":2}`,
			}

			cliConn.CliCommandWithoutTerminalOutputReturns(response, nil)
			_, err := repo.DoesAppExist("app-name")

			Expect(err).To(MatchError("Missing total_results from api response"))
		})

		It("returns an error if the cli response contains a non-number total_results", func() {
			response := []string{
				`{"total_results":"sandwich"}`,
			}

			cliConn.CliCommandWithoutTerminalOutputReturns(response, nil)
			_, err := repo.DoesAppExist("app-name")

			Expect(err).To(MatchError("total_results didn't have a number sandwich"))
		})

		It("returns true if the app exists", func() {
			 response := []string{
			 	`{"total_results":1}`,
			 }
			 spaceGUID := "4"

			 cliConn.CliCommandWithoutTerminalOutputReturns(response, nil)
			 cliConn.GetCurrentSpaceReturns(
			 	plugin_models.Space{
			 		SpaceFields: plugin_models.SpaceFields{
			 			Guid: spaceGUID,
			 		},
			 	},
			 	nil,
			 )

			 result, err := repo.DoesAppExist("app-name")

			 Expect(cliConn.CliCommandWithoutTerminalOutputCallCount()).To(Equal(1))
			 args := cliConn.CliCommandWithoutTerminalOutputArgsForCall(0)
			 Expect(args).To(Equal([]string{"curl", "v2/apps?q=name:app-name&q=space_guid:4"}))

			 Expect(err).ToNot(HaveOccurred())
			 Expect(result).To(BeTrue())
		})

		It("returns false if the app does not exist", func() {
			response := []string{
				`{"total_results":0}`,
			}

			cliConn.CliCommandWithoutTerminalOutputReturns(response, nil)
			result, err := repo.DoesAppExist("app-name")

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeFalse())
		})

	})

	Describe("PushApplication", func() {
		It("pushes an application with both a manifest and a path", func() {
			err := repo.PushApplication("appName", "/path/to/a/manifest.yml", "/path/to/the/app")
			Expect(err).ToNot(HaveOccurred())

			Expect(cliConn.CliCommandCallCount()).To(Equal(1))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{
				"push",
				"appName",
				"-f", "/path/to/a/manifest.yml",
				"-p", "/path/to/the/app",
			}))
		})

		It("pushes an application with only a manifest", func() {
			err := repo.PushApplication("appName", "/path/to/a/manifest.yml", "")
			Expect(err).ToNot(HaveOccurred())

			Expect(cliConn.CliCommandCallCount()).To(Equal(1))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{
				"push",
				"appName",
				"-f", "/path/to/a/manifest.yml",
			}))
		})

		It("returns errors from the push", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("bad app"))

			err := repo.PushApplication("appName", "/path/to/a/manifest.yml", "/path/to/the/app")
			Expect(err).To(MatchError("bad app"))
		})
	})

	Describe("DeleteApplication", func() {
		It("deletes all trace of an application", func() {
			err := repo.DeleteApplication("app-name")
			Expect(err).ToNot(HaveOccurred())

			Expect(cliConn.CliCommandCallCount()).To(Equal(1))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{
				"delete", "app-name",
				"-f",
			}))
		})

		It("returns errors from the delete", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("bad app"))

			err := repo.DeleteApplication("app-name")
			Expect(err).To(MatchError("bad app"))
		})
	})

	Describe("StopApplication", func() {
		It("stops a running application", func() {
			err := repo.StopApplication("app-name")
			Expect(err).ToNot(HaveOccurred())

			Expect(cliConn.CliCommandCallCount()).To(Equal(1))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{
				"stop", "app-name",
			}))
		})

		It("returns errors from the stop", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("bad app"))

			err := repo.StopApplication("app-name")
			Expect(err).To(MatchError("bad app"))
		})
	})

	Describe("StartApplication", func() {
		It("starts a stopped application", func() {
			err := repo.StartApplication("app-name")
			Expect(err).ToNot(HaveOccurred())

			Expect(cliConn.CliCommandCallCount()).To(Equal(1))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{
				"start", "app-name",
			}))
		})

		It("returns errors from the start", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("bad app"))

			err := repo.DeleteApplication("app-name")
			Expect(err).To(MatchError("bad app"))
		})
	})

	Describe("ListApplications", func() {
		It("lists all the applications", func() {
			err := repo.ListApplications()
			Expect(err).ToNot(HaveOccurred())

			Expect(cliConn.CliCommandCallCount()).To(Equal(1))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{"apps"}))
		})

		It("returns errors from the list", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("bad apps"))

			err := repo.ListApplications()
			Expect(err).To(MatchError("bad apps"))
		})
	})

	Describe("MapRoutes", func() {
		It("feeds data to MapRoutesToApp fuction", func() {
			err := repo.MapRoutes("app-name", route)

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("MapRoutesToApp", func() {
		It("adds routes to a specified venerable app", func() {
			err := repo.MapRoutesToApp("app-name", route)
			hostLength := len(route.Host)

			Expect(err).ToNot(HaveOccurred())
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{"map-route", "app-name", "test-domain.com", "--hostname",
				"host-app"}))
			Expect(cliConn.CliCommandCallCount()).To(Equal(hostLength))
		})

		It("returns an error from the MapRoutesToApp with blank route", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("Error mapping routes to venerable app name"))

			err := repo.MapRoutesToApp("app-name", blankRoute)
			Expect(err).To(MatchError("There are no routes to add."))
		})
	})

	Describe("UnmapRoutes", func() {
		It("feeds data to UnmapRoutesFromApp", func() {
			err := repo.UnmapRoutes("app-name", route)

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("UnmapRoutesFromApp", func() {
		It("unmaps routes from a specified application", func() {
			err := repo.UnmapRouteFromApp("app-name", route)
			hostLength := len(route.Host)

			Expect(cliConn.CliCommandCallCount()).To(Equal(hostLength))
			args := cliConn.CliCommandArgsForCall(0)
			Expect(args).To(Equal([]string{"unmap-route", "app-name", "test-domain.com", "--hostname",
				"host-app"}))
			Expect(err).ToNot(HaveOccurred())

		})

		It("returns an error from unmap routes from app when there is no defined route", func() {
			cliConn.CliCommandReturns([]string{}, errors.New("Route could not be unmapped"))

			err := repo.UnmapRouteFromApp("app-name", blankRoute)
			Expect(err).To(MatchError("No routes in the app."))
		})
	})

	Describe("FindUrls", func() {
		It("generates the Urls attached to a specified application", func() {

			appDomainFields := plugin_models.GetApp_DomainFields{
				Guid: "a guid",
				Name: "test-domain.com",
			}

			routesOriginal := plugin_models.GetApp_RouteSummary{
				Guid: "123456789",
				Host: "app-host",
				Domain: appDomainFields,
			}

			routesCopy := plugin_models.GetApp_RouteSummary{
				Guid: "123456987",
				Host: "app-host-copy",
				Domain: appDomainFields,
			}

			cliConn.GetAppReturns(
				plugin_models.GetAppModel{
					Routes: []plugin_models.GetApp_RouteSummary {routesOriginal, routesCopy},
				},
				nil,
			)

			_, err := repo.FindUrls("app-name")

			Expect(err).ToNot(HaveOccurred())
		})

		It("The app entered has no routes", func(){
			_, err := repo.FindUrls("app-name-no-routes")

			Expect(err).To(MatchError("No routes for this app."))
		})
	})
})
