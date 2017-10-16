workspace(name = "com_github_apigee_istio_mixer_adapter")

git_repository(
    name = "io_bazel_rules_go",
    commit = "7991b6353e468ba5e8403af382241d9ce031e571",  # Aug 1, 2017 (gazelle fixes)
    remote = "https://github.com/bazelbuild/rules_go.git",
)

load("@io_bazel_rules_go//go:def.bzl", "go_repositories", "go_repository")

go_repositories()

go_repository(
   name = "com_github_istio_mixer",
   commit = "3e91418c34d4bf92819b06ffcec23d7bcddb889b",
   importpath = "istio.io/mixer",
)

load("@com_github_istio_mixer//:adapter_author_deps.bzl", "mixer_adapter_repositories")

load("@com_github_istio_mixer//:istio_api.bzl", "go_istio_api_repositories")

load("@com_github_istio_mixer//:googleapis.bzl", "go_googleapis_repositories")

load("@com_github_istio_mixer//:x_tools_imports.bzl", "go_x_tools_imports_repositories")

mixer_adapter_repositories()

go_istio_api_repositories()

go_googleapis_repositories()

go_x_tools_imports_repositories()

go_repository(
    name = "com_github_spf13_cobra",
    commit = "35136c09d8da66b901337c6e86fd8e88a1a255bd",  # Jan 30, 2017 (no releases)
    importpath = "github.com/spf13/cobra",
)

go_repository(
    name = "com_github_spf13_pflag",
    commit = "9ff6c6923cfffbcd502984b8e0c80539a94968b7",  # Jan 30, 2017 (no releases)
    importpath = "github.com/spf13/pflag",
)

go_repository(
    name = "com_github_inconshreveable_mousetrap",
    commit = "76626ae9c91c4f2a10f34cad8ce83ea42c93bb75",
    importpath = "github.com/inconshreveable/mousetrap",
)

go_repository(
    name = "org_golang_google_genproto",
    commit = "aa2eb687b4d3e17154372564ad8d6bf11c3cf21f",  # June 1, 2017 (no releases)
    importpath = "google.golang.org/genproto",
)

go_repository(
    name = "com_github_hashicorp_go_multierror",
    commit = "ed905158d87462226a13fe39ddf685ea65f1c11f",  # Dec 16, 2016 (no releases)
    importpath = "github.com/hashicorp/go-multierror",
)

go_repository(
    name = "com_github_hashicorp_errwrap",
    commit = "7554cd9344cec97297fa6649b055a8c98c2a1e55",  # Oct 27, 2014 (no releases)
    importpath = "github.com/hashicorp/errwrap",
)


