package serving

import (
	"sort"
	"strings"

	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/profile"
)

// Gateway objects (feature 009, ADR-0003): Envoy Gateway with the Envoy AI Gateway.
const (
	GatewayNamespace = "aibox-gateway"
	GatewayName      = "aibox"
	RouteName        = "aibox"
	// ClientHeader carries the API key's client id (SecurityPolicy forwardClientIDHeader).
	ClientHeader = "x-aibox-client"
	// ModelHeader is set by the AI Gateway from the request body's `model` field.
	ModelHeader = "x-ai-eg-model"
	// Routing modes.
	RoutingRoundRobin = "round-robin"
	requestTimeout    = "600s"
	tokenCostKey      = "llm_total_token"     //nolint:gosec // metadata key, not a credential
	tokenCostNS       = "io.envoy.ai_gateway" //nolint:gosec // metadata namespace, not a credential
)

// rrServiceName is the headless Service that lists a model's server pods.
func rrServiceName(modelID string) string { return Name(modelID) + "-rr" }

// routing returns the objects that send a model's traffic round-robin across its servers:
// a headless Service (Envoy resolves every pod address and balances per request), an Envoy
// Gateway Backend and an AIServiceBackend.
func routing(m *models.Model) []any {
	name := Name(m.ID)
	labels := map[string]any{"aibox.dev/model": m.ID, "app.kubernetes.io/part-of": "aibox"}
	return []any{
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]any{"name": rrServiceName(m.ID), "namespace": Namespace, "labels": labels},
			"spec": map[string]any{
				"clusterIP": "None",
				"selector":  map[string]any{"aibox.dev/model": m.ID},
				"ports":     []any{map[string]any{"name": "http", "port": Port, "targetPort": Port, "protocol": "TCP"}},
			},
		},
		map[string]any{
			"apiVersion": "gateway.envoyproxy.io/v1alpha1",
			"kind":       "Backend",
			"metadata":   map[string]any{"name": name, "namespace": Namespace, "labels": labels},
			"spec": map[string]any{"endpoints": []any{map[string]any{
				"fqdn": map[string]any{"hostname": rrServiceName(m.ID) + "." + Namespace + ".svc.cluster.local", "port": Port},
			}}},
		},
		map[string]any{
			"apiVersion": "aigateway.envoyproxy.io/v1beta1",
			"kind":       "AIServiceBackend",
			"metadata":   map[string]any{"name": name, "namespace": Namespace, "labels": labels},
			"spec": map[string]any{
				"schema":     map[string]any{"name": "OpenAI"},
				"backendRef": map[string]any{"name": name, "kind": "Backend", "group": "gateway.envoyproxy.io"},
			},
		},
	}
}

// route returns the AIGatewayRoute with one rule per model (matched on the model header) and
// the token usage the rate limits count.
func route(ms []*models.Model) map[string]any {
	var rules []any
	for _, m := range ms {
		rules = append(rules, map[string]any{
			"matches":     []any{map[string]any{"headers": []any{map[string]any{"type": "Exact", "name": ModelHeader, "value": m.ID}}}},
			"backendRefs": []any{map[string]any{"name": Name(m.ID)}},
			"timeouts":    map[string]any{"request": requestTimeout},
		})
	}
	return map[string]any{
		"apiVersion": "aigateway.envoyproxy.io/v1beta1",
		"kind":       "AIGatewayRoute",
		"metadata":   map[string]any{"name": RouteName, "namespace": Namespace, "labels": map[string]any{"app.kubernetes.io/part-of": "aibox"}},
		"spec": map[string]any{
			"parentRefs": []any{map[string]any{
				"name": GatewayName, "namespace": GatewayNamespace, "kind": "Gateway", "group": "gateway.networking.k8s.io",
			}},
			"rules":           rules,
			"llmRequestCosts": []any{map[string]any{"metadataKey": tokenCostKey, "type": "TotalToken"}},
		},
	}
}

// ModelNotFoundBody is the OpenAI-style error the gateway returns for a model without a route.
const ModelNotFoundBody = `{"error":{"message":"The model does not exist or is not served by this AIBox environment.","type":"invalid_request_error","param":"model","code":"model_not_found"}}`

// modelNotFound returns the direct-response filter of the AIGatewayRoute's reserved
// route-not-found rule. The AI Gateway controller creates this filter with a plain-text body
// only when it does not exist yet and never updates it (internal/controller/ai_gateway_route.go
// in v1.2.0), so shipping it under the controller's name gives unknown models an OpenAI-style
// 404. A separate catch-all HTTPRoute cannot do this: it ties with the route-not-found rule and,
// when it wins, also captures /v1/models, which only the AI route's extproc answers.
func modelNotFound() map[string]any {
	return map[string]any{
		"apiVersion": "gateway.envoyproxy.io/v1alpha1",
		"kind":       "HTTPRouteFilter",
		"metadata":   map[string]any{"name": "ai-eg-route-not-found-response-" + RouteName, "namespace": Namespace, "labels": map[string]any{"app.kubernetes.io/part-of": "aibox"}},
		"spec": map[string]any{
			"directResponse": map[string]any{
				"contentType": "application/json",
				"statusCode":  404,
				"body":        map[string]any{"type": "Inline", "inline": ModelNotFoundBody},
			},
		},
	}
}

// rateLimits returns the BackendTrafficPolicy with the profile's per-client requests and
// tokens per minute (global limits, counted in Redis). Clients without an entry share the
// default limit, each with its own counter.
func rateLimits(p *profile.Profile) map[string]any {
	limits := p.Spec.Limits()
	clients := make([]string, 0, len(limits.Clients))
	for c := range limits.Clients {
		clients = append(clients, c)
	}
	sort.Strings(clients)
	var rules []any
	for _, c := range clients {
		sel := []any{map[string]any{"headers": []any{map[string]any{"name": ClientHeader, "type": "Exact", "value": c}}}}
		rules = append(rules, limitRules(sel, limits.Clients[c])...)
	}
	headers := []any{map[string]any{"name": ClientHeader, "type": "Distinct"}}
	if len(clients) > 0 {
		headers = append(headers, map[string]any{
			"name": ClientHeader, "type": "RegularExpression", "value": "^(" + strings.Join(clients, "|") + ")$", "invert": true,
		})
	}
	rules = append(rules, limitRules([]any{map[string]any{"headers": headers}}, limits.Default)...)
	return map[string]any{
		"apiVersion": "gateway.envoyproxy.io/v1alpha1",
		"kind":       "BackendTrafficPolicy",
		"metadata":   map[string]any{"name": "rate-limits", "namespace": GatewayNamespace, "labels": map[string]any{"app.kubernetes.io/part-of": "aibox"}},
		"spec": map[string]any{
			"targetRefs": []any{map[string]any{"group": "gateway.networking.k8s.io", "kind": "Gateway", "name": GatewayName}},
			"rateLimit":  map[string]any{"type": "Global", "global": map[string]any{"rules": rules}},
		},
	}
}

// limitRules returns a requests-per-minute rule and a tokens-per-minute rule whose cost is the
// response's total token count.
func limitRules(selectors []any, l profile.RateLimit) []any {
	return []any{
		map[string]any{
			"clientSelectors": selectors,
			"limit":           map[string]any{"requests": l.RequestsPerMinute, "unit": "Minute"},
		},
		map[string]any{
			"clientSelectors": selectors,
			"limit":           map[string]any{"requests": l.TokensPerMinute, "unit": "Minute"},
			"cost": map[string]any{
				"request":  map[string]any{"from": "Number", "number": 0},
				"response": map[string]any{"from": "Metadata", "metadata": map[string]any{"namespace": tokenCostNS, "key": tokenCostKey}},
			},
		},
	}
}
