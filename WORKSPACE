workspace(name = "com_github_apigee_istio_mixer_adapter")

git_repository(
    name = "io_bazel_rules_go",
    commit = "9cf23e2aab101f86e4f51d8c5e0f14c012c2161c",  # Oct 12, 2017 (Add `build_external` option to `go_repository`)
    remote = "https://github.com/bazelbuild/rules_go.git",
)

load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains")
go_rules_dependencies()
go_register_toolchains(go_version="1.8.3")

load("@io_bazel_rules_go//proto:def.bzl", "proto_register_toolchains")
proto_register_toolchains()

load("@io_bazel_rules_go//go:def.bzl", "go_repository")

go_repository(
   name = "com_github_istio_mixer",
   commit = "0abf3496329789992df39f752a5af8e76686f2c7",
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

# for test...

go_repository(
    name = "com_github_hashicorp_golang_lru",
    commit = "0a025b7e63adc15a622f29b0b2c4c3848243bbf6",  # Aug 13, 2016 (no releases)
    importpath = "github.com/hashicorp/golang-lru",
)

go_repository(
    name = "com_github_prometheus_client_golang",
    commit = "c5b7fccd204277076155f10851dad72b76a49317",  # Aug 17, 2016 (v0.8.0)
    importpath = "github.com/prometheus/client_golang",
)

go_repository(
    name = "com_github_grpc_ecosystem_go_grpc_prometheus",
    commit = "2500245aa6110c562d17020fb31a2c133d737799",  # Mar 30, 2017 (only 1 release)
    importpath = "github.com/grpc-ecosystem/go-grpc-prometheus",
)

go_repository(
    name = "com_github_opentracing_opentracing_go",
    commit = "1949ddbfd147afd4d964a9f00b24eb291e0e7c38",  # May 3, 2017 (v1.0.2)
    importpath = "github.com/opentracing/opentracing-go",
)

go_repository(
    name = "com_github_openzipkin_zipkin_go_opentracing",
    commit = "90d57f421daae5e385ce2429580f0d695c41823b",  # Jul 5, 2017 (has releases but we need a newer commit)
    importpath = "github.com/openzipkin/zipkin-go-opentracing",
    build_file_proto_mode = "legacy",
)

go_repository(
    name = "com_github_grpc_ecosystem_grpc_opentracing",
    commit = "6c130eed1e297e1aa4d415a50c90d0c81c52677e",  # Feb 24, 2017 (no releases)
    importpath = "github.com/grpc-ecosystem/grpc-opentracing",
)

go_repository(
    name = "com_github_ghodss_yaml",
    commit = "04f313413ffd65ce25f2541bfd2b2ceec5c0908c",  # Dec 6, 2016 (no releases)
    importpath = "github.com/ghodss/yaml",
    build_file_proto_mode = "legacy",
)

go_repository(
    name = "com_github_grpc_ecosystem_go_grpc_middleware",
    commit = "f63a7dfb64c138bd93d5c5b896d8b33c4b08e000",  # Jun 11, 2017 (no releases)
    importpath = "github.com/grpc-ecosystem/go-grpc-middleware",
)

go_repository(
    name = "com_github_prometheus_common",
    commit = "dd2f054febf4a6c00f2343686efb775948a8bff4",  # Jan 8, 2017 (no releases)
    importpath = "github.com/prometheus/common",
)

go_repository(
    name = "com_github_emicklei_go_restful",
    commit = "09691a3b6378b740595c1002f40c34dd5f218a22",  # Dec 12, 2016 (k8s deps)
    importpath = "github.com/emicklei/go-restful",
)

go_repository(
    name = "in_gopkg_yaml_v2",
    commit = "14227de293ca979cf205cd88769fe71ed96a97e2",  # Jan 24, 2017 (no releases)
    importpath = "gopkg.in/yaml.v2",
)

go_repository(
    name = "com_github_opentracing_contrib_go_observer",
    commit = "a52f2342449246d5bcc273e65cbdcfa5f7d6c63c",  # June 20, 2017 (no release)
    importpath = "github.com/opentracing-contrib/go-observer",
)

go_repository(
    name = "com_github_Shopify_sarama",
    commit = "c01858abb625b73a3af51d0798e4ad42c8147093",  # May 8, 2017 (1.12.0)
    importpath = "github.com/Shopify/sarama",
)

go_repository(
    name = "com_github_apache_thrift",
    build_file_name = "BUILD.bazel",
    commit = "b2a4d4ae21c789b689dd162deb819665567f481c",  # Jan 6, 2017 (0.10.0)
    importpath = "github.com/apache/thrift",
)

go_repository(
    name = "com_github_go_logfmt_logfmt",
    commit = "390ab7935ee28ec6b286364bba9b4dd6410cb3d5",  # Nov 15, 2016 (0.3.0)
    importpath = "github.com/go-logfmt/logfmt",
)

go_repository(
    name = "com_github_prometheus_procfs",
    commit = "1878d9fbb537119d24b21ca07effd591627cd160",  # Jan 28, 2017 (no releases)
    importpath = "github.com/prometheus/procfs",
)

go_repository(
    name = "com_github_prometheus_client_model",
    commit = "fa8ad6fec33561be4280a8f0514318c79d7f6cb6",  # Feb 12, 2015 (only release too old)
    importpath = "github.com/prometheus/client_model",
)

go_repository(
    name = "com_github_prometheus_client_golang",
    commit = "c5b7fccd204277076155f10851dad72b76a49317",  # Aug 17, 2016 (v0.8.0)
    importpath = "github.com/prometheus/client_golang",
)

go_repository(
    name = "com_github_beorn7_perks",
    commit = "4c0e84591b9aa9e6dcfdf3e020114cd81f89d5f9",  # Aug 4, 2016 (no releases)
    importpath = "github.com/beorn7/perks",
)

go_repository(
    name = "com_github_matttproud_golang_protobuf_extensions",
    commit = "c12348ce28de40eed0136aa2b644d0ee0650e56c",  # Apr 24, 2016 (v1.0.0)
    importpath = "github.com/matttproud/golang_protobuf_extensions",
)

go_repository(
    name = "io_k8s_apimachinery",
    build_file_generation = "on",
    build_file_name = "BUILD.bazel",
    build_file_proto_mode = "legacy",
    commit = "6134cb2da6d90597b0434e349f90f94fafc9ae51",  # Jun 19, 2017 (no releases)
    importpath = "k8s.io/apimachinery",
)

go_repository(
    name = "io_k8s_client_go",
    build_file_generation = "on",
    build_file_name = "BUILD.bazel",
    build_file_proto_mode = "legacy",
    commit = "7c69e980210777a6292351ac6873de083526f08e",  # Jul 18, 2017 (no releases)
    importpath = "k8s.io/client-go",
)

go_repository(
    name = "com_github_pierrec_lz4",
    commit = "88df27974e3644957507a1ca1866edc8e98d4897",  # May 11, 2017 (no releases)
    importpath = "github.com/pierrec/lz4",
)

go_repository(
    name = "com_github_eapache_go_resiliency",
    commit = "6800482f2c813e689c88b7ed3282262385011890",  # Feb 13, 2015 (1.0.0)
    importpath = "github.com/eapache/go-resiliency",
)

go_repository(
    name = "com_github_rcrowley_go_metrics",
    commit = "1f30fe9094a513ce4c700b9a54458bbb0c96996c",  # Nov 28, 2016 (no releases)
    importpath = "github.com/rcrowley/go-metrics",
)

go_repository(
    name = "com_github_eapache_go_xerial_snappy",
    commit = "bb955e01b9346ac19dc29eb16586c90ded99a98c",  # June 9, 2016 (no releases)
    importpath = "github.com/eapache/go-xerial-snappy",
)

go_repository(
    name = "com_github_eapache_queue",
    commit = "ded5959c0d4e360646dc9e9908cff48666781367",  # June 6, 2017 (1.0.2)
    importpath = "github.com/eapache/queue",
)

go_repository(
    name = "com_github_davecgh_go_spew",
    commit = "346938d642f2ec3594ed81d874461961cd0faa76",  # Nov 14, 2016 (1.1.0)
    importpath = "github.com/davecgh/go-spew",
)

go_repository(
    name = "com_github_golang_snappy",
    commit = "553a641470496b2327abcac10b36396bd98e45c9",  # Feb 15, 2017 (no releases)
    importpath = "github.com/golang/snappy",
)

go_repository(
    name = "com_github_pierrec_xxHash",
    commit = "f051bb7f1d1aaf1b5a665d74fb6b0217712c69f7",  # March 20, 2016 (0.1.1)
    importpath = "github.com/pierrec/xxHash",
)

go_repository(
    name = "org_golang_x_oauth2",
    commit = "3c3a985cb79f52a3190fbc056984415ca6763d01",  # Aug 26, 2016 (no releases)
    importpath = "golang.org/x/oauth2",
)

go_repository(
    name = "com_github_howeyc_gopass",
    commit = "bf9dde6d0d2c004a008c27aaee91170c786f6db8",  # Jan 9, 2017 (no releases)
    importpath = "github.com/howeyc/gopass",
)

go_repository(
    name = "com_github_go_openapi_spec",
    commit = "6aced65f8501fe1217321abf0749d354824ba2ff",  # Aug 8, 2016 (no releases)
    importpath = "github.com/go-openapi/spec",
)


go_repository(
    name = "com_github_google_gofuzz",
    commit = "44d81051d367757e1c7c6a5a86423ece9afcf63c",  # Nov 22, 2016 (no releases)
    importpath = "github.com/google/gofuzz",
)


go_repository(
    name = "com_github_imdario_mergo",
    commit = "3e95a51e0639b4cf372f2ccf74c86749d747fbdc",  # Feb 16, 2016 (v0.2.2)
    importpath = "github.com/imdario/mergo",
)


go_repository(
    name = "com_github_googleapis_gnostic",
    commit = "0c5108395e2debce0d731cf0287ddf7242066aba",  # Jul 29, 2017 (no releases)
    importpath = "github.com/googleapis/gnostic",
    build_file_proto_mode = "legacy",
)

go_repository(
    name = "io_k8s_api",
    build_file_generation = "on",
    build_file_name = "BUILD.bazel",
    build_file_proto_mode = "legacy",
    commit = "4d5cc6efc5e84aa19fb1bd3f911c16a6723c1bb7",  # Jul 19, 2017 (no releases)
    importpath = "k8s.io/api",
)

go_repository(
    name = "com_github_emicklei_go_restful_swagger12",
    commit = "dcef7f55730566d41eae5db10e7d6981829720f6",  # Feb 8, 2017 (no releases)
    importpath = "github.com/emicklei/go-restful-swagger12",
)

go_repository(
    name = "com_github_go_openapi_swag",
    commit = "1d0bd113de87027671077d3c71eb3ac5d7dbba72",  # Jul 4, 2016 (no releases)
    importpath = "github.com/go-openapi/swag",
)

go_repository(
    name = "in_gopkg_inf_v0",
    commit = "3887ee99ecf07df5b447e9b00d9c0b2adaa9f3e4",  # Sep 11, 2015 (latest commit)
    importpath = "gopkg.in/inf.v0",
)

go_repository(
    name = "com_github_go_openapi_jsonpointer",
    commit = "46af16f9f7b149af66e5d1bd010e3574dc06de98",  # Jul 4, 2016 (no releases)
    importpath = "github.com/go-openapi/jsonpointer",
)


go_repository(
    name = "com_github_go_openapi_jsonreference",
    commit = "13c6e3589ad90f49bd3e3bbe2c2cb3d7a4142272",  # Jul 4, 2016 (no releases)
    importpath = "github.com/go-openapi/jsonreference",
)

go_repository(
    name = "com_github_mailru_easyjson",
    commit = "2af9a745a611440bab0528e5ac19b2805a1c50eb",  # Mar 28, 2017 (no releases)
    importpath = "github.com/mailru/easyjson",
)

go_repository(
    name = "com_github_PuerkitoBio_purell",
    commit = "0bcb03f4b4d0a9428594752bd2a3b9aa0a9d4bd4",  # Nov 14, 2016 (v1.1.0)
    importpath = "github.com/PuerkitoBio/purell",
)

go_repository(
    name = "com_github_PuerkitoBio_urlesc",
    commit = "bbf7a2afc14f93e1e0a5c06df524fbd75e5031e5",  # Mar 24, 2017 (no releases)
    importpath = "github.com/PuerkitoBio/urlesc",
)

go_repository(
    name = "com_github_pborman_uuid",
    commit = "a97ce2ca70fa5a848076093f05e639a89ca34d06",  # Feb 9, 2016 (v1.0)
    importpath = "github.com/pborman/uuid",
)

go_repository(
    name = "com_google_cloud_go",
    commit = "a5913b3f7deecba45e98ff33cefbac4fd204ddd7",  # Jun 27, 2017 (v0.10.0)
    importpath = "cloud.google.com/go",
)

go_repository(
    name = "com_github_coreos_pkg",
    commit = "1c941d73110817a80b9fa6e14d5d2b00d977ce2a",  # Feb 6, 2017 (fix for build dir bazel issue)
    importpath = "github.com/coreos/pkg",
)

go_repository(
    name = "com_github_jonboulle_clockwork",
    commit = "2eee05ed794112d45db504eb05aa693efd2b8b09",  # Jul 6, 2016 (v0.1.0)
    importpath = "github.com/jonboulle/clockwork",
)

go_repository(
    name = "com_github_imdario_mergo",
    commit = "3e95a51e0639b4cf372f2ccf74c86749d747fbdc",  # Feb 16, 2016 (v0.2.2)
    importpath = "github.com/imdario/mergo",
)

go_repository(
    name = "com_github_howeyc_gopass",
    commit = "bf9dde6d0d2c004a008c27aaee91170c786f6db8",  # Jan 9, 2017 (no releases)
    importpath = "github.com/howeyc/gopass",
)

go_repository(
    name = "org_golang_x_crypto",
    commit = "cbc3d0884eac986df6e78a039b8792e869bff863",  # Apr 8, 2017 (no releases)
    importpath = "golang.org/x/crypto",
)

go_repository(
    name = "com_github_googleapis_gax_go",
    commit = "9af46dd5a1713e8b5cd71106287eba3cefdde50b",  # Mar 20, 2017 (no releases)
    importpath = "github.com/googleapis/gax-go",
)

go_repository(
    name = "com_github_ugorji_go",
    commit = "708a42d246822952f38190a8d8c4e6b16a0e600c",  # Mar 12, 2017 (no releases)
    importpath = "github.com/ugorji/go",
)

go_repository(
    name = "in_gopkg_inf_v0",
    commit = "3887ee99ecf07df5b447e9b00d9c0b2adaa9f3e4",  # Sep 11, 2015 (latest commit)
    importpath = "gopkg.in/inf.v0",
)

go_repository(
    name = "com_github_docker_distribution",
    commit = "a25b9ef0c9fe242ac04bb20d3a028442b7d266b6",  # Apr 5, 2017 (v2.6.1)
    importpath = "github.com/docker/distribution",
)

go_repository(
    name = "com_github_go_openapi_spec",
    commit = "6aced65f8501fe1217321abf0749d354824ba2ff",  # Aug 8, 2016 (no releases)
    importpath = "github.com/go-openapi/spec",
)

go_repository(
    name = "com_github_google_gofuzz",
    commit = "44d81051d367757e1c7c6a5a86423ece9afcf63c",  # Nov 22, 2016 (no releases)
    importpath = "github.com/google/gofuzz",
)

go_repository(
    name = "com_github_googleapis_gnostic",
    commit = "0c5108395e2debce0d731cf0287ddf7242066aba",  # Jul 29, 2017 (no releases)
    importpath = "github.com/googleapis/gnostic",
    build_file_proto_mode = "legacy",
)

go_repository(
    name = "com_github_emicklei_go_restful",
    commit = "09691a3b6378b740595c1002f40c34dd5f218a22",  # Dec 12, 2016 (k8s deps)
    importpath = "github.com/emicklei/go-restful",
)

go_repository(
    name = "com_github_juju_ratelimit",
    commit = "5b9ff866471762aa2ab2dced63c9fb6f53921342",  # May 23, 2017 (no releases)
    importpath = "github.com/juju/ratelimit",
)

go_repository(
    name = "com_github_opencontainers_go_digest",
    commit = "aa2ec055abd10d26d539eb630a92241b781ce4bc",  # Jan 31, 2017 (v1.0.0-rc0)
    importpath = "github.com/opencontainers/go-digest",
)

go_repository(
    name = "com_github_blang_semver",
    commit = "b38d23b8782a487059e8fc8773e9a5b228a77cb6",  # Jan 30, 2017 (v3.5.0)
    importpath = "github.com/blang/semver",
)