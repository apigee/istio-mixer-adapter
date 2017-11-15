load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "verify_api_key.go",
        "verify_api_key_structs.go",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "@io_istio_istio//mixer/pkg/adapter:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    size="small",
    srcs = [
        "verify_api_key_test.go",
    ],
    importpath = "github.com/istio-mixer-adapter/apigee/auth",
    library = ":go_default_library",
    deps = [
        "//apigee/testutil:go_default_library",
        "@io_istio_istio//mixer/pkg/adapter:go_default_library",
    ],
)
