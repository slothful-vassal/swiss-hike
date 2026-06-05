module github.com/slothful-vassal/wanderer-plugin-swiss-hike

go 1.25.0

require (
	github.com/extism/go-pdk v1.1.3
	github.com/open-wanderer/wanderer/plugins/sdk v0.0.0
	golang.org/x/net v0.53.0
)

replace github.com/open-wanderer/wanderer/plugins/sdk => ../../wanderer/plugins/sdk
