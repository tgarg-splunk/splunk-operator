// Copyright (c) 2018-2020 Splunk Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package enterprise

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	enterprisev1 "github.com/splunk/splunk-operator/pkg/apis/enterprise/v1alpha3"
	splcommon "github.com/splunk/splunk-operator/pkg/splunk/common"
	splctrl "github.com/splunk/splunk-operator/pkg/splunk/controller"
)

// kubernetes logger used by splunk.enterprise package
var log = logf.Log.WithName("splunk.enterprise")

// ApplySplunkConfig reconciles the state of Kubernetes Secrets, ConfigMaps and other general settings for Splunk Enterprise instances.
func ApplySplunkConfig(client splcommon.ControllerClient, cr splcommon.MetaObject, spec enterprisev1.CommonSplunkSpec, instanceType InstanceType) (*corev1.Secret, error) {
	var err error

	// Creates/updates the namespace scoped "splunk-secrets" K8S secret object
	namespaceScopedSecret, err := ApplyNamespaceScopedSecretObject(client, cr.GetNamespace())
	if err != nil {
		return nil, err
	}

	// create splunk defaults (for inline config)
	if spec.Defaults != "" {
		defaultsMap := getSplunkDefaults(cr.GetName(), cr.GetNamespace(), instanceType, spec.Defaults)
		defaultsMap.SetOwnerReferences(append(defaultsMap.GetOwnerReferences(), splcommon.AsOwner(cr)))
		if err = splctrl.ApplyConfigMap(client, defaultsMap); err != nil {
			return nil, err
		}
	}

	return namespaceScopedSecret, nil
}

// GetSmartstoreRemoteVolumeSecrets is used to retrieve S3 access key and secrete keys.
// To do: sgontla:
// 1. Support multiple secret objects
// 2. default secret object
// volume name other than default, look for the specific secret object, else fetch from defaults
func GetSmartstoreRemoteVolumeSecrets(volume enterprisev1.VolumeSpec, client splcommon.ControllerClient, cr splcommon.MetaObject, smartstore *enterprisev1.SmartStoreSpec) (string, string, string, error) {
	namespaceScopedSecret, err := GetNamespaceScopedSecretByName(client, cr.GetNamespace(), volume.SecretRef)
	if err != nil {
		return "", "", "", err
	}

	accessKey := string(namespaceScopedSecret.Data[s3AccessKey])
	secretKey := string(namespaceScopedSecret.Data[s3SecretKey])

	if accessKey == "" {
		return "", "", "", fmt.Errorf("S3 Access Key is missing")
	} else if secretKey == "" {
		return "", "", "", fmt.Errorf("S3 Secret Key is missing")
	}

	return accessKey, secretKey, namespaceScopedSecret.ResourceVersion, nil
}

// CreateSmartStoreConfigMap creates the configMap with Smartstore config in INI format
func CreateSmartStoreConfigMap(client splcommon.ControllerClient, cr splcommon.MetaObject,
	smartstore *enterprisev1.SmartStoreSpec, serverConf *enterprisev1.ServerConfSpec) (*corev1.ConfigMap, error) {

	var crKind string
	crKind = cr.GetObjectKind().GroupVersionKind().Kind

	scopedLog := log.WithName("CreateSmartStoreConfigMap").WithValues("kind", crKind, "name", cr.GetName(), "namespace", cr.GetNamespace())

	// 1. Prepare the indexes.conf entries

	mapSplunkConfDetails := make(map[string]string)
	// Get the list of volumes in INI format
	volumesConfIni, err := GetSmartstoreVolumesConfig(client, cr, smartstore, mapSplunkConfDetails)
	if err != nil {
		return nil, err
	}

	if volumesConfIni == "" {
		scopedLog.Info("Volume stanza list is empty")
	}

	// Get the list of indexes in INI format
	indexesConfIni := GetSmartstoreIndexesConfig(smartstore.IndexList)

	if indexesConfIni == "" {
		scopedLog.Info("Index stanza list is empty")
	} else if volumesConfIni == "" {
		return nil, fmt.Errorf("Indexes without Volume configuration is not allowed")
	}

	defaultsConfIni := GetSmartstoreIndexesDefaults(smartstore.Defaults)

	iniSmartstoreConf := fmt.Sprintf(`%s %s %s`, defaultsConfIni, volumesConfIni, indexesConfIni)
	mapSplunkConfDetails["indexes.conf"] = iniSmartstoreConf
	// 2. Prepare server.conf entries

	if serverConf != nil {
		iniServerConf := GetServerConfigEntries(serverConf)
		mapSplunkConfDetails["server.conf"] = iniServerConf
	}

	// Create smartstore config consisting indexes.conf
	SplunkOperatorAppConfigMap := prepareSplunkSmartstoreConfigMap(cr.GetName(), cr.GetNamespace(), crKind, mapSplunkConfDetails)

	SplunkOperatorAppConfigMap.SetOwnerReferences(append(SplunkOperatorAppConfigMap.GetOwnerReferences(), splcommon.AsOwner(cr)))
	if err := splctrl.ApplyConfigMap(client, SplunkOperatorAppConfigMap); err != nil {
		return nil, err
	}

	return SplunkOperatorAppConfigMap, nil
}

//  setupInitContainer modifies the podTemplateSpec object to incorporate support for DFS.
func setupInitContainer(podTemplateSpec *corev1.PodTemplateSpec, sparkImage string, imagePullPolicy string) {
	// create an init container in the pod, which is just used to populate the jdk and spark mount directories
	containerSpec := corev1.Container{
		Image:           sparkImage,
		ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
		Name:            "init",
		// Do not change the ln options:
		// n=don't chase directory contents in case of a directory, f=force creat
		Command: []string{"bash", "-c", "mkdir -p /op/spl/et/apps/ && ln -sfn  /mnt/splunk-operator /op/spl/et/apps/splunk-operator"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "pvc-etc", MountPath: "/op/spl/et"},
		},

		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("0.25"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
	}
	podTemplateSpec.Spec.InitContainers = append(podTemplateSpec.Spec.InitContainers, containerSpec)
}
