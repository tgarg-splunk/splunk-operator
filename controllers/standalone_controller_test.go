package controllers

import (
	"context"
	"time"

	enterprisev4 "github.com/splunk/splunk-operator/api/v4"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	splcommon "github.com/splunk/splunk-operator/pkg/splunk/common"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const timeout = time.Second * 120
const interval = time.Second * 2

var _ = Describe("Standalone Controller", func() {

	var (
		namespace = "ns-splunk"
	)

	BeforeEach(func() {
		time.Sleep(5 * time.Second)
	})

	AfterEach(func() {

	})

	Context("Standalone Management", func() {

		It("Create Standalone custom resource should succeeded", func() {
			nsSpecs := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			Expect(k8sClient.Create(context.Background(), nsSpecs)).Should(Succeed())
			ss := CreateStandlaone("test-ss1", namespace, splcommon.PhaseReady)
			Expect(string(splcommon.PhaseReady)).To(Equal(ss.Status.Phase))
			DeleteStandalone("test-ss1", namespace)
		})

	})
})

func callUnsedMethods(rr *StandaloneReconciler, namespace string) {
	key := types.NamespacedName{
		Name:      "secret5",
		Namespace: namespace,
	}

	secret := &corev1.Secret{}
	_ = k8sClient.Get(context.TODO(), key, secret)
}

func CreateStandlaone(name string, namespace string, status splcommon.Phase) *enterprisev4.Standalone {
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	ssSpec := &enterprisev4.Standalone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: enterprisev4.StandaloneSpec{},
	}

	Expect(k8sClient.Create(context.Background(), ssSpec)).Should(Succeed())
	time.Sleep(5 * time.Second)

	By("Expecting Standalone custom resource to be created successfully")
	ss := &enterprisev4.Standalone{}
	Eventually(func() bool {
		_ = k8sClient.Get(context.Background(), key, ss)
		if status != "" {
			ss.Status.Phase = status
			Expect(k8sClient.Status().Update(context.Background(), ss)).Should(Succeed())
			time.Sleep(5 * time.Second)
		}
		return true
	}, timeout, interval).Should(BeTrue())

	return ss
}

func DeleteStandalone(name string, namespace string) {
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	By("Expecting Standalone Deleted successfully")
	Eventually(func() error {
		ssys := &enterprisev4.Standalone{}
		_ = k8sClient.Get(context.Background(), key, ssys)
		err := k8sClient.Delete(context.Background(), ssys)
		return err
	}, timeout, interval).Should(Succeed())
}
