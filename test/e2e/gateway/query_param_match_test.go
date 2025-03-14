// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build e2e
// +build e2e

package gateway

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/projectcontour/contour/internal/gatewayapi"
	"github.com/projectcontour/contour/test/e2e"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapi_v1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func testGatewayQueryParamMatch(namespace string) {
	Specify("query param matching works", func() {
		t := f.T()

		f.Fixtures.Echo.Deploy(namespace, "echo-1")
		f.Fixtures.Echo.Deploy(namespace, "echo-2")
		f.Fixtures.Echo.Deploy(namespace, "echo-3")
		f.Fixtures.Echo.Deploy(namespace, "echo-4")

		route := &gatewayapi_v1beta1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "httproute-1",
			},
			Spec: gatewayapi_v1beta1.HTTPRouteSpec{
				Hostnames: []gatewayapi_v1beta1.Hostname{"queryparams.gateway.projectcontour.io"},
				CommonRouteSpec: gatewayapi_v1beta1.CommonRouteSpec{
					ParentRefs: []gatewayapi_v1beta1.ParentReference{
						gatewayapi.GatewayParentRef("", "http"),
					},
				},
				Rules: []gatewayapi_v1beta1.HTTPRouteRule{
					{
						Matches: []gatewayapi_v1beta1.HTTPRouteMatch{
							{QueryParams: gatewayapi.HTTPQueryParamMatches(map[string]string{"animal": "whale"})},
						},
						BackendRefs: gatewayapi.HTTPBackendRef("echo-1", 80, 1),
					},
					{
						Matches: []gatewayapi_v1beta1.HTTPRouteMatch{
							{QueryParams: gatewayapi.HTTPQueryParamMatches(map[string]string{"animal": "dolphin"})},
						},
						BackendRefs: gatewayapi.HTTPBackendRef("echo-2", 80, 1),
					},
					{
						Matches: []gatewayapi_v1beta1.HTTPRouteMatch{
							{QueryParams: gatewayapi.HTTPQueryParamMatches(map[string]string{"animal": "dolphin", "color": "red"})},
						},
						BackendRefs: gatewayapi.HTTPBackendRef("echo-3", 80, 1),
					},
					{
						Matches:     gatewayapi.HTTPRouteMatch(gatewayapi_v1beta1.PathMatchPathPrefix, "/"),
						BackendRefs: gatewayapi.HTTPBackendRef("echo-4", 80, 1),
					},
				},
			},
		}
		f.CreateHTTPRouteAndWaitFor(route, httpRouteAccepted)

		cases := map[string]string{
			"/?animal=whale":                     "echo-1",
			"/?animal=whale&foo=bar":             "echo-1", // extra irrelevant parameters have no impact
			"/?animal=whale&animal=dolphin":      "echo-1", // the first occurrence of a given key in a querystring is used for matching
			"/?animal=dolphin":                   "echo-2",
			"/?animal=dolphin&animal=whale":      "echo-2", // the first occurrence of a given key in a querystring is used for matching
			"/?animal=dolphin&color=blue":        "echo-2", // all matches must match for a route to be selected
			"/?animal=dolphin&color=red":         "echo-3",
			"/?animal=dolphin&color=red&foo=bar": "echo-3", // extra irrelevant parameters have no impact
			"/?animal=horse":                     "echo-4", // non-matching values do not match
			"/?animal=whalesay":                  "echo-4", // value matching is exact, not prefix
			"/?animal=bluedolphin":               "echo-4", // value matching is exact, not suffix
			"/?color=blue":                       "echo-4",
			"/?nomatch=true":                     "echo-4",
		}

		for path, expectedService := range cases {
			t.Logf("Querying %q, expecting service %q", path, expectedService)

			res, ok := f.HTTP.RequestUntil(&e2e.HTTPRequestOpts{
				Host:      string(route.Spec.Hostnames[0]),
				Path:      path,
				Condition: e2e.HasStatusCode(200),
			})
			if !assert.Truef(t, ok, "expected 200 response code, got %d", res.StatusCode) {
				continue
			}

			body := f.GetEchoResponseBody(res.Body)
			assert.Equal(t, namespace, body.Namespace)
			assert.Equal(t, expectedService, body.Service)
		}
	})
}
