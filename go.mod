module github.com/immanent-tech/foragd

go 1.25.1

replace github.com/immanent-tech/go-syndication v0.0.0 => ./pkg/go-syndication

replace github.com/immanent-tech/slog-elasticsearch v0.0.0 => ./pkg/slog-elasticsearch

require (
	github.com/a-h/templ v0.3.943
	github.com/alexedwards/scs/v2 v2.9.0
	github.com/angelofallars/htmx-go v0.5.0
	github.com/auth0/go-auth0 v1.28.0
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-playground/form/v4 v4.2.1
	github.com/go-playground/validator/v10 v10.27.0
	github.com/knadh/koanf/providers/env v1.1.0
	github.com/knadh/koanf/providers/file v1.2.0
	github.com/knadh/koanf/v2 v2.3.0
	github.com/lmittmann/tint v1.1.2
	github.com/mattn/go-isatty v0.0.20
	github.com/oapi-codegen/runtime v1.1.2
	github.com/rs/cors v1.11.1
	github.com/samber/slog-chi v1.16.1
	github.com/samber/slog-multi v1.5.0
	golang.org/x/oauth2 v0.31.0
)

require (
	github.com/Masterminds/semver v0.0.0-20190925130524-317e8cce5480 // indirect
	github.com/Masterminds/vcs v1.13.3 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de // indirect
	github.com/armon/go-radix v0.0.0-20180808171621-7fddfc383310 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/common-nighthawk/go-figure v0.0.0-20200609044655-c4b36f998cf2 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.7.0 // indirect
	github.com/go-http-utils/fresh v0.0.0-20161124030543-7231e26a4b27 // indirect
	github.com/go-http-utils/headers v0.0.0-20181008091004-fed159eddc2a // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-json-experiment/json v0.0.0-20250714165856-be8212f5270d // indirect
	github.com/go-pkgz/expirable-cache/v3 v3.0.0 // indirect
	github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c // indirect
	github.com/gofri/go-github-ratelimit/v2 v2.0.2 // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/golang/dep v0.5.4 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-github/v30 v30.1.0 // indirect
	github.com/google/go-github/v74 v74.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jedib0t/go-pretty/v6 v6.0.4 // indirect
	github.com/jmank88/nuts v0.4.0 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/nightlyone/lockfile v1.0.0 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/oapi-codegen/oapi-codegen/v2 v2.5.0 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/onsi/gomega v1.34.1 // indirect
	github.com/package-url/packageurl-go v0.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/recoilme/pudge v1.0.3 // indirect
	github.com/rhysd/go-github-selfupdate v1.2.3 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/samber/slog-common v0.19.0 // indirect
	github.com/sdboyer/constext v0.0.0-20170321163424-836a14457353 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/sonatype-nexus-community/go-sona-types v0.1.6 // indirect
	github.com/sonatype-nexus-community/nancy v1.0.51 // indirect
	github.com/speakeasy-api/jsonpath v0.6.0 // indirect
	github.com/spf13/afero v1.1.2 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/jwalterweatherman v1.0.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/spf13/viper v1.7.1 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/tcnksm/go-gitconfig v0.1.2 // indirect
	github.com/ulikunitz/xz v0.5.14 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)

require (
	github.com/PuerkitoBio/rehttp v1.4.0 // indirect
	github.com/alecthomas/kong v1.12.1
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/coreos/go-oidc/v3 v3.15.0
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/didip/tollbooth/v8 v8.0.1
	github.com/dprotaso/go-yit v0.0.0-20220510233725-9ba8df137936 // indirect
	github.com/elastic/go-elasticsearch/v9 v9.1.0
	github.com/fatih/color v1.18.0
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/getkin/kin-openapi v0.132.0 // indirect
	github.com/go-http-utils/etag v0.0.0-20161124023236-513ea8f21eb1
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-resty/resty/v2 v2.16.5 // indirect
	github.com/go-shiori/go-readability v0.0.0-20250217085726-9f5bf5ca7612
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goforj/godump v1.6.0
	github.com/gohugoio/hashstructure v0.5.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-github/v75 v75.0.0
	github.com/google/uuid v1.6.0 // indirect
	github.com/immanent-tech/go-syndication v0.0.0
	github.com/immanent-tech/slog-elasticsearch v0.0.0
	github.com/jferrl/go-githubauth v1.4.2
	github.com/josharian/intern v1.0.0 // indirect
	github.com/justinas/alice v1.2.0
	github.com/justinas/nosurf v1.2.0
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/parsers/toml/v2 v2.2.0
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lestrrat-go/blackmagic v1.0.3 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc v1.0.6 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/jwx/v2 v2.1.6 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matoous/go-nanoid/v2 v2.1.0
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/realclientip/realclientip-go v1.0.0
	github.com/reugn/go-quartz v0.15.2
	github.com/samber/lo v1.51.0 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/speakeasy-api/openapi-overlay v0.10.2 // indirect
	github.com/veqryn/slog-context v0.8.0
	github.com/veqryn/slog-json v0.5.0
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/net v0.42.0
	golang.org/x/sync v0.17.0
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool (
	github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
	github.com/sonatype-nexus-community/nancy
	golang.org/x/tools/cmd/stringer
)
