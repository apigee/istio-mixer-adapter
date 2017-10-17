package(default_visibility = ["//visibility:public"])

#load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

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
        "@com_github_istio_mixer//template/metric:go_default_library",
        "@com_github_istio_mixer//template/quota:go_default_library",
    ],
)

#go_test(
#    name = "go_default_test",
#    size = "medium",
#    srcs = ["apigee_test.go"],
#    library = ":go_default_library",
#    data = [
#        "sampleoperatorconfig",
#    ],
#    deps = [
#        "//pkg/adapter:go_default_library",
#        "//pkg/adapter/test:go_default_library",
#        "//template/logentry:go_default_library",
#        "//template/metric:go_default_library",
#        "//template/quota:go_default_library",
#        "@com_github_googleapis_googleapis//:google/rpc",
#        "//test/testenv:go_default_library",
#        "//template:go_default_library",
#        "@com_github_istio_api//:mixer/v1",  # keep
#        "@org_golang_x_net//context:go_default_library",
#        "//template/auth:go_default_library",
#    ],
#)
