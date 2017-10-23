package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "apigee.go",
    ],
    deps = [
        "//apigee/analytics:go_default_library",
        "//apigee/auth:go_default_library",
        "//apigee/config:go_default_library",
        "//template/auth:go_default_library",
        "@com_github_gogo_protobuf//types:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
        "@com_github_istio_mixer//pkg/adapter:go_default_library",
        "@com_github_istio_mixer//template/logentry:go_default_library",
        "@com_github_istio_mixer//template/quota:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    size = "medium",
    srcs = [
        "apigee_test.go"
    ],
    library = ":go_default_library",
    data = [
        "//testdata"
    ],
    deps = [
        "@io_istio_api//:mixer/v1",  # keep
        "@org_golang_x_net//context:go_default_library",
        "@com_github_istio_mixer//template:go_default_library",
        "@com_github_istio_mixer//test/testenv:go_default_library",
        "@com_github_istio_mixer//pkg/adapter:go_default_library",
        "@com_github_istio_mixer//pkg/template:go_default_library",

        "//apigee/analytics:go_default_library",
        "//apigee/auth:go_default_library",
        "//apigee/config:go_default_library",
        "//template/auth:go_default_library",
        "@com_github_gogo_protobuf//types:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
        "@com_github_istio_mixer//template/logentry:go_default_library",
        "@com_github_istio_mixer//template/quota:go_default_library",
    ],
)
