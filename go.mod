module github.com/splunk/splunk-operator

go 1.16

require (
	cloud.google.com/go v0.99.0 // indirect
	github.com/aws/aws-sdk-go v1.42.16
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/go-logr/logr v1.2.0
	github.com/google/go-cmp v0.5.6
	github.com/minio/minio-go/v7 v7.0.16
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	go.uber.org/zap v1.19.1
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/kubectl v0.23.1
	sigs.k8s.io/controller-runtime v0.11.0
)
