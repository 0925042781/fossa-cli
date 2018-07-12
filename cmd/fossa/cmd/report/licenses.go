package report

import (
	"fmt"
	"text/template"

	"github.com/fossas/fossa-cli/cmd/fossa/cmdutil"
	"github.com/urfave/cli"

	"github.com/fossas/fossa-cli/api/fossa"
	"github.com/fossas/fossa-cli/cmd/fossa/flags"
	"github.com/fossas/fossa-cli/log"
)

const defaultLicenseReportTemplate = `# 3rd-Party Software License Notice
Generated by fossa-cli (https://github.com/fossas/fossa-cli).
This software includes the following software and licenses:
{{range $license, $deps := .}}
========================================================================
{{$license}}
========================================================================
The following software have components provided under the terms of this license:
{{range $i, $dep := $deps}}
- {{$dep.Project.Title}} (from {{$dep.Project.URL}})
{{- end}}
{{end}}
`

var licensesCmd = cli.Command{
	Name:  "licenses",
	Usage: "Generate licenses report",
	Flags: flags.WithGlobalFlags(flags.WithAPIFlags(flags.WithModulesFlags(flags.WithReportTemplateFlags([]cli.Flag{
		// TODO: what does this actually do?
		cli.BoolFlag{Name: flags.Short(Unknown), Usage: "include unknown licenses"},
	})))),
	Action: licensesRun,
}

func licensesRun(ctx *cli.Context) (err error) {
	analyzed, err := analyzeModules(ctx)
	if err != nil {
		log.Logger.Fatal("Could not analyze modules: %s", err.Error())
	}

	defer log.StopSpinner()
	revs := make([]fossa.Revision, 0)
	for _, module := range analyzed {
		if ctx.Bool(Unknown) {
			totalDeps := len(module.Deps)
			i := 0
			for _, dep := range module.Deps {
				i++
				log.ShowSpinner(fmt.Sprintf("Fetching License Info (%d/%d): %s", i, totalDeps, dep.ID.Name))
				locator := fossa.LocatorOf(dep.ID)
				// Quirk of the licenses API: Go projects are stored under the git fetcher.
				if locator.Fetcher == "go" {
					locator.Fetcher = "git"
				}
				rev, err := fossa.GetRevision(fossa.LocatorOf(dep.ID))
				if err != nil {
					log.Logger.Warning(err.Error())
					continue
				}
				revs = append(revs, rev)
			}
		} else {
			log.ShowSpinner("Fetching License Info")
			var locators []fossa.Locator
			for _, dep := range module.Deps {
				locator := fossa.LocatorOf(dep.ID)
				if locator.Fetcher == "go" {
					locator.Fetcher = "git"
				}
				locators = append(locators, locator)
			}
			revs, err = fossa.GetRevisions(locators)
			if err != nil {
				log.Logger.Fatalf("Could not fetch revisions: %s", err.Error())
			}
		}
	}
	log.StopSpinner()

	depsByLicense := make(map[string]map[string]fossa.Revision, 0)
	for _, rev := range revs {
		for _, license := range rev.Licenses {
			if _, ok := depsByLicense[license.LicenseID]; !ok {
				depsByLicense[license.LicenseID] = make(map[string]fossa.Revision, 0)
			}
			depsByLicense[license.LicenseID][rev.Locator.String()] = rev
		}
	}

	if tmplFile := ctx.String(flags.Template); tmplFile != "" {
		err := cmdutil.OutputWithTemplateFile(tmplFile, depsByLicense)
		if err != nil {
			log.Logger.Fatalf("Could not parse template data: %s", err.Error())
		}
		return nil
	}

	tmpl, err := template.New("base").Parse(defaultLicenseReportTemplate)
	if err != nil {
		log.Logger.Fatalf("Could not parse template data: %s", err.Error())
	}

	return cmdutil.OutputWithTemplate(tmpl, depsByLicense)
}
