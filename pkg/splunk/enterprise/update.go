package enterprise

import (
	"context"
	enteprise_v1alpha12 "git.splunk.com/splunk-operator/pkg/apis/enterprise/v1alpha1"
	"k8s.io/api/apps/v1"
	api_core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	typed_core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func UpdateDeployment(cr *enteprise_v1alpha12.SplunkEnterprise, client client.Client) error {

	err := UpdateCluster(cr, client)
	if err != nil {
		return err
	}

	return nil
}


func UpdateCluster(cr *enteprise_v1alpha12.SplunkEnterprise, client client.Client) error {

	err := UpdateSearchHeads(cr, client)
	if err != nil {
		return err
	}

	return nil
}


func UpdateSearchHeads(cr *enteprise_v1alpha12.SplunkEnterprise, client client.Client) error {

	sts := v1.StatefulSet{
		TypeMeta: meta_v1.TypeMeta{
			Kind: "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name: GetSplunkStatefulsetName(SPLUNK_SEARCH_HEAD, GetIdentifier(cr)),
			Namespace: cr.Namespace,
		},
	}

	namespacedName := types.NamespacedName{
		Namespace: sts.Namespace,
		Name: sts.Name,
	}

	err :=  client.Get(context.TODO(), namespacedName, &sts)
	if err != nil {
		return err
	}

	oldSearchHeadCount := int(*sts.Spec.Replicas)

	if oldSearchHeadCount != cr.Spec.SearchHeads {
		replicas := int32(cr.Spec.SearchHeads)
		sts.Spec.Replicas = &replicas

		err = client.Update(context.TODO(), &sts)
		if err != nil {
			return err
		}

		go AddNewMembersToCluster(cr, oldSearchHeadCount, cr.Spec.SearchHeads, client)
	}

	return nil
}


func AddNewMembersToCluster(cr *enteprise_v1alpha12.SplunkEnterprise, prevMemCount, newMemCount int, client client.Client) {
	log.Println("Trying to make an exec call...")

	coreClient := typed_core_v1.CoreV1Client{}

	for i := prevMemCount; i < newMemCount; i++ {
		log.Println("Calling for index %d\n", i)

		numTries := 60
		waitTime := 1 * time.Second

		for j := 0; j < numTries; j++ {
			log.Println("Checking if Pod exists...")
			pod := api_core_v1.Pod{
				TypeMeta: meta_v1.TypeMeta{
					Kind: "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name: GetSplunkStatefulsetPodName(SPLUNK_SEARCH_HEAD, GetIdentifier(cr), i),
					Namespace: cr.Namespace,
				},
			}
			namespacedName := types.NamespacedName{
				Namespace: pod.Namespace,
				Name: pod.Name,
			}
			err :=  client.Get(context.TODO(), namespacedName, &pod)
			if err == nil && pod.Status.Phase == api_core_v1.PodRunning {
				break
			} else {
				log.Println("Pod is not yet running...")
				time.Sleep(waitTime)
			}
		}

		execCommand := coreClient.RESTClient().Post().
			Resource("pods").
			Name(GetSplunkStatefulsetPodName(SPLUNK_SEARCH_HEAD, GetIdentifier(cr), i)).
			Namespace(cr.Namespace).
			SubResource("exec").
			Param("container", "splunk").
			Param("command", "echo 'Testing'")

		log.Println("Sending call...")
		result := execCommand.Do()
		log.Println("Received call.")

		body, err := result.Raw()
		if err != nil {
			log.Println(err)
		} else {
			log.Println(body)
		}
	}
	log.Println("Done.")
}