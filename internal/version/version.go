package version

// Version is the panel semver. Release builds overwrite it via -ldflags from
// the VERSION file / git tag. Unset local builds stay "dev".
var Version = "dev"
