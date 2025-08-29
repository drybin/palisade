module github.com/drybin/palisade

go 1.22.3

require (
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0
	github.com/go-resty/resty/v2 v2.16.5
	github.com/joho/godotenv v1.5.1
	github.com/stretchr/testify v1.10.0
	github.com/urfave/cli/v2 v2.27.7
	github.com/ztrue/tracerr v0.4.0
	mexc-sdk/mexcsdk v0.0.0
)

replace mexc-sdk/mexcsdk => ./pkg/mexcsdk

require (
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/aws/jsii-runtime-go v1.44.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/net v0.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
