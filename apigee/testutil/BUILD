load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "mockenv.go",
        "apikey.go",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//apigee/auth:go_default_library",
        "@io_istio_istio//mixer/pkg/adapter:go_default_library",
    ],
)
