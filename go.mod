module github.com/immanent-tech/foragd

go 1.26

replace github.com/immanent-tech/go-syndication v0.0.0 => ./pkg/go-syndication

replace github.com/immanent-tech/slog-elasticsearch v0.0.0 => ./pkg/slog-elasticsearch

replace github.com/immanent-tech/slog-chi v0.0.0 => ./pkg/slog-chi

replace github.com/dprotaso/go-yit v0.0.0-20260209000607-dfb86291624d => github.com/dprotaso/go-yit v0.0.0-20250513224043-18a80f8f6df4

require (
	cloud.google.com/go/pubsub/v2 v2.4.0
	github.com/ThreeDotsLabs/watermill v1.5.1
	github.com/a-h/templ v0.3.1001
	github.com/alexedwards/scs/v2 v2.9.0
	github.com/angelofallars/htmx-go v0.5.0
	github.com/elastic/elastic-transport-go/v8 v8.9.0
	github.com/go-chi/chi/v5 v5.2.5
	github.com/go-json-experiment/json v0.0.0-20260214004413-d219187c3433
	github.com/go-playground/form/v4 v4.3.0
	github.com/go-playground/validator/v10 v10.30.1
	github.com/goforj/godump v1.9.1
	github.com/gofri/go-github-ratelimit/v2 v2.0.2
	github.com/googleapis/gax-go/v2 v2.18.0
	github.com/knadh/koanf/v2 v2.3.3
	github.com/lmittmann/tint v1.1.3
	github.com/mattn/go-isatty v0.0.20
	github.com/oapi-codegen/runtime v1.2.0
	github.com/samber/slog-multi v1.7.1
	github.com/stripe/stripe-go/v83 v83.2.1
	github.com/zeebo/xxh3 v1.1.0
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.18.0
	go.opentelemetry.io/otel/sdk v1.42.0
	go.opentelemetry.io/otel/sdk/metric v1.42.0
	golang.org/x/oauth2 v0.36.0
	google.golang.org/genproto v0.0.0-20260226221140-a57be14db171
	google.golang.org/grpc v1.79.3
)

require (
	cel.dev/expr v0.25.1 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.18.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.31.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.55.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.55.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.37.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/go-pkgz/expirable-cache/v3 v3.1.0 // indirect
	github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.14 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/lithammer/shortuuid/v3 v3.0.7 // indirect
	github.com/matoous/go-nanoid/v2 v2.1.0 // indirect
	github.com/oapi-codegen/oapi-codegen/v2 v2.6.0 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/samber/slog-common v0.20.0 // indirect
	github.com/speakeasy-api/jsonpath v0.6.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	github.com/woodsbury/decimal128 v1.4.0 // indirect
	go.devnw.com/structs v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.42.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.67.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.67.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.42.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	go.uber.org/nilaway v0.0.0-20260213150243-937701de96c7 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
	google.golang.org/api v0.271.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

require (
	cloud.google.com/go/billing v1.21.0
	cloud.google.com/go/errorreporting v0.4.0
	cloud.google.com/go/storage v1.61.3
	codeberg.org/readeck/go-readability/v2 v2.1.1
	github.com/BurntSushi/toml v1.6.0
	github.com/PuerkitoBio/rehttp v1.4.0 // indirect
	github.com/alecthomas/kong v1.14.0
	github.com/auth0/go-auth0/v2 v2.7.0
	github.com/coreos/go-oidc/v3 v3.17.0
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1 // indirect
	github.com/didip/tollbooth/v8 v8.0.1
	github.com/dprotaso/go-yit v0.0.0-20260209000607-dfb86291624d // indirect
	github.com/elastic/go-elasticsearch/v9 v9.3.1
	github.com/fatih/color v1.18.0
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/getkin/kin-openapi v0.133.0 // indirect
	github.com/go-chi/cors v1.2.2
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-resty/resty/v2 v2.17.2
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/google/go-github/v75 v75.0.0
	github.com/google/uuid v1.6.0 // indirect
	github.com/immanent-tech/go-syndication v0.0.0
	github.com/immanent-tech/slog-chi v0.0.0
	github.com/jferrl/go-githubauth v1.5.1
	github.com/josharian/intern v1.0.0 // indirect
	github.com/justinas/alice v1.2.0
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/env/v2 v2.0.0
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lestrrat-go/blackmagic v1.0.4 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc v1.0.6 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/jwx/v2 v2.1.6 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/realclientip/realclientip-go v1.0.0
	github.com/resend/resend-go/v3 v3.1.1
	github.com/reugn/go-quartz v0.15.2
	github.com/riandyrn/otelchi v0.12.2
	github.com/samber/lo v1.53.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/speakeasy-api/openapi-overlay v0.10.3 // indirect
	github.com/stripe/stripe-go v70.15.0+incompatible
	github.com/veqryn/slog-context v0.9.0
	github.com/veqryn/slog-context/otel v0.9.0
	github.com/veqryn/slog-json v0.5.0
	github.com/yuin/goldmark v1.7.16
	go.abhg.dev/goldmark/frontmatter v0.3.0
	go.opentelemetry.io/otel v1.42.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.42.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.42.0
	go.opentelemetry.io/otel/log v0.18.0
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.18.0
	go.opentelemetry.io/otel/trace v1.42.0
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/net v0.52.0
	golang.org/x/sync v0.20.0
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool (
	github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
	go.uber.org/nilaway/cmd/nilaway
	golang.org/x/tools/cmd/stringer
)
