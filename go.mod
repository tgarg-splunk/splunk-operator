module github.com/splunk/splunk-operator

go 1.16

require (
	github.com/aws/aws-sdk-go v1.42.16
	github.com/go-logr/logr v1.2.2
	github.com/google/go-cmp v0.5.6
	github.com/minio/minio-go/v7 v7.0.16
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	go.uber.org/zap v1.19.1
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/kubectl v0.22.4
	knative.dev/pkg v0.0.0-20220112031350-b90b853c5212
	sigs.k8s.io/controller-runtime v0.10.0
)

replace (
	github.com/go-logr/logr v1.2.2 => github.com/go-logr/logr v0.4.0
	go.uber.org/zap v1.19.1 => go.uber.org/zap v1.19.0
	k8s.io/api v0.22.5 => k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.5 => k8s.io/apimachinery v0.22.4
	k8s.io/client-go v0.22.5 => k8s.io/client-go v0.22.4
	knative.dev/pkg v0.0.0-20220112031350-b90b853c5212 => knative.dev/pkg v0.0.0-20210706174620-fe90576475ca
)
