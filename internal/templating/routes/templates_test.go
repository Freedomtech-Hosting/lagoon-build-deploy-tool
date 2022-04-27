package routes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/uselagoon/lagoon-routegen/internal/helpers"
	"github.com/uselagoon/lagoon-routegen/internal/lagoon"
)

func TestGenerateKubeTemplate(t *testing.T) {
	type args struct {
		route                  lagoon.RouteV2
		values                 lagoon.BuildValues
		monitoringContact      string
		monitoringStatusPageID string
		monitoringEnabled      bool
		activeStandby          bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "active-standby1",
			args: args{
				route: lagoon.RouteV2{
					Domain:         "extra-long-name.a-really-long-name-that-should-truncate.www.example.com",
					Service:        "nginx",
					MonitoringPath: "/",
					Insecure:       helpers.StrPtr("Redirect"),
					HSTS:           helpers.StrPtr("null"),
					TLSAcme:        helpers.BoolPtr(true),
					Migrate:        helpers.BoolPtr(true),
					Annotations: map[string]string{
						"custom-annotation": "custom annotation value",
					},
					Fastly: lagoon.Fastly{
						Watch: false,
					},
				},
				values: lagoon.BuildValues{
					Project:         "example-project",
					Environment:     "environment-with-really-really-reall-3fdb",
					EnvironmentType: "development",
					Namespace:       "myexample-project-environment-with-really-really-reall-3fdb",
					BuildType:       "branch",
					LagoonVersion:   "v2.x.x",
					Kubernetes:      "lagoon.local",
					Branch:          "environment-with-really-really-reall-3fdb",
				},
				monitoringContact:      "abcdefg",
				monitoringStatusPageID: "12345",
				monitoringEnabled:      true,
				activeStandby:          true,
			},
			want: `---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    custom-annotation: custom annotation value
    fastly.amazee.io/watch: "false"
    ingress.kubernetes.io/ssl-redirect: "true"
    kubernetes.io/tls-acme: "true"
    lagoon.sh/branch: environment-with-really-really-reall-3fdb
    lagoon.sh/version: v2.x.x
    monitor.stakater.com/enabled: "true"
    monitor.stakater.com/overridePath: /
    nginx.ingress.kubernetes.io/server-snippet: |
      add_header X-Robots-Tag "noindex, nofollow";
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    uptimerobot.monitor.stakater.com/alert-contacts: abcdefg
    uptimerobot.monitor.stakater.com/interval: "60"
    uptimerobot.monitor.stakater.com/status-pages: "12345"
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: custom-ingress
    dioscuri.amazee.io/migrate: "true"
    helm.sh/chart: custom-ingress-0.1.0
    lagoon.sh/autogenerated: "false"
    lagoon.sh/buildType: branch
    lagoon.sh/environment: environment-with-really-really-reall-3fdb
    lagoon.sh/environmentType: development
    lagoon.sh/project: example-project
    lagoon.sh/service: extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    lagoon.sh/service-type: custom-ingress
  name: extra-long-name-f6c8a
spec:
  rules:
  - host: extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    http:
      paths:
      - backend:
          service:
            name: nginx
            port:
              name: http
        path: /
        pathType: Prefix
  tls:
  - hosts:
    - extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    secretName: extra-long-name-f6c8a-tls
status:
  loadBalancer: {}
`,
		},
		{
			name: "custom-ingress1",
			args: args{
				route: lagoon.RouteV2{
					Domain:         "extra-long-name.a-really-long-name-that-should-truncate.www.example.com",
					Service:        "nginx",
					MonitoringPath: "/",
					Insecure:       helpers.StrPtr("Redirect"),
					HSTS:           helpers.StrPtr("null"),
					TLSAcme:        helpers.BoolPtr(true),
					Migrate:        helpers.BoolPtr(false),
					Annotations: map[string]string{
						"custom-annotation": "custom annotation value",
					},
					Fastly: lagoon.Fastly{
						Watch: false,
					},
				},
				values: lagoon.BuildValues{
					Project:         "example-project",
					Environment:     "environment-with-really-really-reall-3fdb",
					EnvironmentType: "development",
					Namespace:       "myexample-project-environment-with-really-really-reall-3fdb",
					BuildType:       "branch",
					LagoonVersion:   "v2.x.x",
					Kubernetes:      "lagoon.local",
					Branch:          "environment-with-really-really-reall-3fdb",
				},
				monitoringContact:      "abcdefg",
				monitoringStatusPageID: "12345",
				monitoringEnabled:      true,
				activeStandby:          false,
			},
			want: `---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    custom-annotation: custom annotation value
    fastly.amazee.io/watch: "false"
    ingress.kubernetes.io/ssl-redirect: "true"
    kubernetes.io/tls-acme: "true"
    lagoon.sh/branch: environment-with-really-really-reall-3fdb
    lagoon.sh/version: v2.x.x
    monitor.stakater.com/enabled: "true"
    monitor.stakater.com/overridePath: /
    nginx.ingress.kubernetes.io/server-snippet: |
      add_header X-Robots-Tag "noindex, nofollow";
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    uptimerobot.monitor.stakater.com/alert-contacts: abcdefg
    uptimerobot.monitor.stakater.com/interval: "60"
    uptimerobot.monitor.stakater.com/status-pages: "12345"
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: custom-ingress
    dioscuri.amazee.io/migrate: "false"
    helm.sh/chart: custom-ingress-0.1.0
    lagoon.sh/autogenerated: "false"
    lagoon.sh/buildType: branch
    lagoon.sh/environment: environment-with-really-really-reall-3fdb
    lagoon.sh/environmentType: development
    lagoon.sh/project: example-project
    lagoon.sh/service: extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    lagoon.sh/service-type: custom-ingress
  name: extra-long-name-f6c8a
spec:
  rules:
  - host: extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    http:
      paths:
      - backend:
          service:
            name: nginx
            port:
              name: http
        path: /
        pathType: Prefix
  tls:
  - hosts:
    - extra-long-name.a-really-long-name-that-should-truncate.www.example.com
    secretName: extra-long-name-f6c8a-tls
status:
  loadBalancer: {}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateKubeTemplate(tt.args.route, tt.args.values, tt.args.monitoringContact, tt.args.monitoringStatusPageID, tt.args.monitoringEnabled)
			if !reflect.DeepEqual(string(got), tt.want) {
				t.Errorf("GenerateKubeTemplate() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestReadValuesFile(t *testing.T) {
	type args struct {
		file string
	}
	tests := []struct {
		name string
		args args
		want lagoon.BuildValues
	}{
		{
			name: "branch-values",
			args: args{
				file: "test-resources/values.yaml",
			},
			want: lagoon.BuildValues{
				Project:                         "myexample-project",
				Environment:                     "environment-with-really-really-reall-3fdb",
				EnvironmentType:                 "development",
				Namespace:                       "myexample-project-environment-with-really-really-reall-3fdb",
				BuildType:                       "branch",
				LagoonVersion:                   "v2.x.x",
				Kubernetes:                      "lagoon.local",
				Branch:                          "environment-with-really-really-reall-3fdb",
				RoutesAutogeneratePrefixes:      []string{"www"},
				RoutesAutogenerateInsecure:      "true",
				RoutesAutogenerateEnabled:       "true",
				RoutesAutogeneratePrefixHyphens: "false",
			},
		},
		{
			name: "pullrequest-values",
			args: args{
				file: "test-resources/pr-values.yaml",
			},
			want: lagoon.BuildValues{
				Project:                         "myexample-project",
				Environment:                     "environment-with-really-really-reall-3fdb",
				EnvironmentType:                 "development",
				Namespace:                       "myexample-project-environment-with-really-really-reall-3fdb",
				BuildType:                       "branch",
				LagoonVersion:                   "v2.x.x",
				Kubernetes:                      "lagoon.local",
				PRNumber:                        "1234",
				PRHeadBranch:                    "main",
				PRBaseBranch:                    "main",
				RoutesAutogeneratePrefixes:      []string{"www"},
				RoutesAutogenerateInsecure:      "true",
				RoutesAutogenerateEnabled:       "true",
				RoutesAutogeneratePrefixHyphens: "false",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReadValuesFile(tt.args.file); !reflect.DeepEqual(got, tt.want) {
				stra, _ := json.Marshal(got)
				strb, _ := json.Marshal(tt.want)
				fmt.Println(string(stra))
				fmt.Println(string(strb))
				t.Errorf("ReadValuesFile() = %v, want %v", got, tt.want)
			}
		})
	}
}