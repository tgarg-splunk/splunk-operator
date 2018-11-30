package stub

import (
	"fmt"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"operator/splunk-operator/pkg/apis/splunk-instance/v1alpha1"
	"operator/splunk-operator/pkg/stub/splunk"
	"time"
)

func UpdateDeployment(cr *v1alpha1.SplunkInstance) error {

	err := UpdateCluster(cr)
	if err != nil {
		return err
	}

	return nil
}


func UpdateCluster(cr *v1alpha1.SplunkInstance) error {

	err := UpdateSearchHeads(cr)
	if err != nil {
		return err
	}

	return nil
}


func UpdateSearchHeads(cr *v1alpha1.SplunkInstance) error {

	sts := v1.StatefulSet{
		TypeMeta: meta_v1.TypeMeta{
			Kind: "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name: splunk.GetSplunkStatefulsetName(splunk.SPLUNK_SEARCH_HEAD, splunk.GetIdentifier(cr)),
			Namespace: cr.Namespace,
		},
	}

	err := sdk.Get(&sts)
	if err != nil {
		return err
	}

	oldSearchHeadCount := int(*sts.Spec.Replicas)

	if oldSearchHeadCount != cr.Spec.SearchHeads {
		replicas := int32(cr.Spec.SearchHeads)
		sts.Spec.Replicas = &replicas

		err = sdk.Update(&sts)
		if err != nil {
			return err
		}

		go AddNewMembersToCluster(cr, oldSearchHeadCount, cr.Spec.SearchHeads)
	}

	return nil
}


func AddNewMembersToCluster(cr *v1alpha1.SplunkInstance, prevMemCount, newMemCount int) {
	fmt.Println("Trying to make an exec call...")

	client := k8sclient.GetKubeClient()
	fmt.Println("Got the kube client.")

	for i := prevMemCount; i < newMemCount; i++ {
		fmt.Printf("Calling for index %d\n", i)

		numTries := 60
		waitTime := 1 * time.Second

		for j := 0; j < numTries; j++ {
			fmt.Println("Checking if Pod exists...")
			pod := core_v1.Pod{
				TypeMeta: meta_v1.TypeMeta{
					Kind: "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name: splunk.GetSplunkStatefulsetPodName(splunk.SPLUNK_SEARCH_HEAD, splunk.GetIdentifier(cr), i),
					Namespace: cr.Namespace,
				},
			}
			err := sdk.Get(&pod)
			if err == nil && pod.Status.Phase == core_v1.PodRunning {
				break
			} else {
				fmt.Println("Pod is not yet running...")
				time.Sleep(waitTime)
			}
		}

		execCommand := client.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(splunk.GetSplunkStatefulsetPodName(splunk.SPLUNK_SEARCH_HEAD, splunk.GetIdentifier(cr), i)).
			Namespace(cr.Namespace).
			SubResource("exec").
			Param("container", "splunk").
			Param("command", "echo 'Testing'")

		fmt.Println("Sending call...")
		result := execCommand.Do()
		fmt.Println("Received call.")

		body, err := result.Raw()
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(body)
		}
	}
	fmt.Println("Done.")
}