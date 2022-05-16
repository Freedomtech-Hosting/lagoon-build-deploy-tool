package cmd

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uselagoon/build-deploy-tool/internal/helpers"
	"github.com/uselagoon/build-deploy-tool/internal/lagoon"
	routeTemplater "github.com/uselagoon/build-deploy-tool/internal/templating/routes"
)

var supportedAutogeneratedTypes = []string{
	// "kibana", //@TODO: don't even need this anymore?
	"node",
	"node-persistent",
	"nginx",
	"nginx-php",
	"nginx-php-persistent",
	"varnish",
	"varnish-persistent",
	"python-persistent",
	"python",
}

var autogenRouteGeneration = &cobra.Command{
	Use:     "autogenerated-ingress",
	Aliases: []string{"autogen-ingress", "autogen", "ai"},
	Short:   "Generate the autogenerated ingress templates for a Lagoon build",
	RunE: func(cmd *cobra.Command, args []string) error {
		return AutogeneratedIngressGeneration(true)
	},
}

// AutogeneratedIngressGeneration handles generating autogenerated ingress
func AutogeneratedIngressGeneration(debug bool) error {
	activeEnv := false
	standbyEnv := false

	lagoonEnvVars := []lagoon.EnvironmentVariable{}
	lagoonValues := lagoon.BuildValues{}
	lYAML := lagoon.YAML{}
	lCompose := lagoon.Compose{}
	err := collectBuildValues(debug, &activeEnv, &standbyEnv, &lagoonEnvVars, &lagoonValues, &lYAML, &lCompose)
	if err != nil {
		return err
	}

	autogenRoutes, err := generateAutogenRoutes(lagoonEnvVars, lYAML, lagoonValues, lCompose)
	if err != nil {
		return err
	}

	// generate the templates
	for _, route := range autogenRoutes.Routes {
		// autogenerated routes use the `servicename` as the name of the ingress resource, use `IngressName` in routev2 to handle this
		if debug {
			fmt.Println(fmt.Sprintf("Templating autogenerated ingress manifest for %s to %s", route.Domain, fmt.Sprintf("%s/%s.yaml", savedTemplates, route.IngressName)))
		}
		templateYAML := routeTemplater.GenerateIngressTemplate(route, lagoonValues, monitoringContact, monitoringStatusPageID, false)
		routeTemplater.WriteTemplateFile(fmt.Sprintf("%s/%s.yaml", savedTemplates, route.IngressName), templateYAML)
	}

	return nil
}

func generateAutogenRoutes(
	envVars []lagoon.EnvironmentVariable,
	lagoonYAML lagoon.YAML,
	lagoonValues lagoon.BuildValues,
	lagoonCompose lagoon.Compose,
) (*lagoon.RoutesV2, error) {
	autogenEnabled := true
	// check if autogenerated routes are disabled
	if lagoonYAML.Routes.Autogenerate.Enabled != nil {
		if *lagoonYAML.Routes.Autogenerate.Enabled == false {
			autogenEnabled = false
		}
	}
	// check if pullrequests autogenerated routes are disabled
	if lagoonValues.BuildType == "pullrequest" && lagoonYAML.Routes.Autogenerate.AllowPullRequests != nil {
		if *lagoonYAML.Routes.Autogenerate.AllowPullRequests == false {
			autogenEnabled = false
		} else {
			autogenEnabled = true
		}
	}
	// check if this environment has autogenerated routes disabled
	if lagoonYAML.Environments[lagoonValues.Branch].AutogenerateRoutes != nil {
		if *lagoonYAML.Environments[lagoonValues.Branch].AutogenerateRoutes == false {
			autogenEnabled = false
		} else {
			autogenEnabled = true
		}
	}
	// generate autogenerated routes for the services
	autogenRoutes := &lagoon.RoutesV2{}

	// get the router pattern
	lagoonRouterPattern, err := lagoon.GetLagoonVariable("LAGOON_SYSTEM_ROUTER_PATTERN", []string{"internal_system"}, envVars)
	if err == nil {
		// if the `LAGOON_SYSTEM_ROUTER_PATTERN` exists, generate the routes
		for serviceName, service := range lagoonCompose.Services {
			// get the service type
			serviceType := lagoon.CheckServiceLagoonLabel(service.Labels, "lagoon.type")

			// check if this service has autogenerated routes disabled
			serviceAutogenerated := lagoon.CheckServiceLagoonLabel(service.Labels, "lagoon.autogeneratedroute")
			if reflect.TypeOf(serviceAutogenerated).Kind() == reflect.String {
				vBool, err := strconv.ParseBool(serviceAutogenerated)
				if err == nil {
					autogenEnabled = vBool
				}
			}

			// if autogenerated routes are enabled, generate them :)
			if autogenEnabled {
				if helpers.Contains(supportedAutogeneratedTypes, serviceType) {
					domain, shortDomain := AutogeneratedDomainFromPattern(lagoonRouterPattern.Value, serviceName)
					serviceValues := lagoon.ServiceValues{
						AutogeneratedRouteDomain:      domain,
						ShortAutogeneratedRouteDomain: shortDomain,
					}
					lagoonValues.Services[serviceName] = serviceValues

					// alternativeNames are `prefixes` for autogenerated routes
					autgenPrefixes := lagoonYAML.Routes.Autogenerate.Prefixes
					alternativeNames := []string{}
					for _, altName := range autgenPrefixes {
						// add the prefix to the domain into a new slice of alternative domains
						alternativeNames = append(alternativeNames, fmt.Sprintf("%s.%s", altName, domain))
					}
					fastlyConfig := &lagoon.Fastly{}
					err := lagoon.GenerateFastlyConfiguration(fastlyConfig, fastlyCacheNoCahce, fastlyServiceID, domain, fastlyAPISecretPrefix, envVars)
					if err != nil {
						return autogenRoutes, err
					}
					insecure := "Allow"
					if lagoonYAML.Routes.Autogenerate.Insecure != "" {
						insecure = lagoonYAML.Routes.Autogenerate.Insecure
					}
					autogenRoute := lagoon.RouteV2{
						Domain:  domain,
						Fastly:  *fastlyConfig,
						TLSAcme: helpers.BoolPtr(true),
						// overwrite the custom-ingress labels
						Labels: map[string]string{
							"lagoon.sh/autogenerated":    "true",
							"helm.sh/chart":              fmt.Sprintf("%s-%s", "autogenerated-ingress", "0.1.0"),
							"app.kubernetes.io/name":     "autogenerated-ingress",
							"app.kubernetes.io/instance": serviceName,
							"lagoon.sh/service":          serviceName,
							"lagoon.sh/service-type":     serviceType,
						},
						Service:          serviceName,
						IngressName:      serviceName,
						Insecure:         &insecure,
						AlternativeNames: alternativeNames,
					}
					autogenRoutes.Routes = append(autogenRoutes.Routes, autogenRoute)
				}
			}
		}
		return autogenRoutes, nil
	}
	return autogenRoutes, nil
}

// AutogeneratedDomainFromPattern generates the domain name and the shortened domain name for an autogenerated ingress
func AutogeneratedDomainFromPattern(pattern, service string) (string, string) {
	domain := pattern
	shortDomain := pattern

	// fallback check for ${service} in the router pattern
	hasServicePattern := false
	if strings.Contains(pattern, "${service}") {
		hasServicePattern = true
	}

	// find and replace
	domain = strings.Replace(domain, "${service}", service, 1)
	domain = strings.Replace(domain, "${project}", projectName, 1)
	domain = strings.Replace(domain, "${environment}", environmentName, 1)
	// find and replace for the short domain
	shortDomain = strings.Replace(shortDomain, "${service}", service, 1)
	shortDomain = strings.Replace(shortDomain, "${project}", helpers.GetBase32EncodedLowercase(helpers.GetSha256Hash(projectName))[:8], 1)
	shortDomain = strings.Replace(shortDomain, "${environment}", helpers.GetBase32EncodedLowercase(helpers.GetSha256Hash(environmentName))[:8], 1)

	if !hasServicePattern {
		domain = fmt.Sprintf("%s.%s", service, domain)
		shortDomain = fmt.Sprintf("%s.%s", service, shortDomain)
	}

	domainParts := strings.Split(domain, ".")
	domainHash := helpers.GetSha256Hash(domain)
	finalDomain := ""
	for count, part := range domainParts {
		domainPart := part
		if len(part) > 63 {
			domainPart = fmt.Sprintf("%s-%s", part[:54], domainHash[:8])
		}
		if count == (len(domainParts) - 1) {
			finalDomain = fmt.Sprintf("%s%s", finalDomain, domainPart)
		} else {
			finalDomain = fmt.Sprintf("%s%s.", finalDomain, domainPart)
		}
	}
	return finalDomain, shortDomain
}

func init() {
	templateCmd.AddCommand(autogenRouteGeneration)
}
