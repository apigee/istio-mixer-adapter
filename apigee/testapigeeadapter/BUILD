package(default_visibility = ["//visibility:public"])

load("@com_github_istio_mixer//tools/codegen:generate.bzl", "mixer_supported_template_library")

mixer_supported_template_library(
    name = "go_default_library",
    packages = {
        "//template/auth:go_default_library_proto.descriptor_set": "github.com/apigee/istio-mixer-adapter/template/auth",
        "//template/analytics:go_default_library_proto.descriptor_set": "github.com/apigee/istio-mixer-adapter/template/analytics",
    },
    deps = [
        "//template/auth:go_default_library",
        "//template/analytics:go_default_library",
    ],
)
