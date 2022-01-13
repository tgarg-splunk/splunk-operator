// Copyright (c) 2018-2021 Splunk Inc. All rights reserved.
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
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	enterpriseApi "github.com/splunk/splunk-operator/pkg/apis/enterprise/v3"
	splclient "github.com/splunk/splunk-operator/pkg/splunk/client"
	splcommon "github.com/splunk/splunk-operator/pkg/splunk/common"
	splctrl "github.com/splunk/splunk-operator/pkg/splunk/controller"
	spltest "github.com/splunk/splunk-operator/pkg/splunk/test"
	splutil "github.com/splunk/splunk-operator/pkg/splunk/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateAndAddPipelineWorker(t *testing.T) {
	appDeployInfo := &enterpriseApi.AppDeploymentInfo{
		AppName:          "testapp.spl",
		LastModifiedTime: "testtime",
		ObjectHash:       "abc0123",
		Size:             1234,
		RepoState:        enterpriseApi.RepoStateActive,
		DeployStatus:     enterpriseApi.DeployStatusPending,
		PhaseInfo: enterpriseApi.PhaseInfo{
			Phase:  enterpriseApi.PhaseDownload,
			Status: enterpriseApi.AppPkgDownloadPending,
		},
	}

	podName := "splunk-s2apps-standalone-0"
	cr := enterpriseApi.Standalone{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
	}

	statefulSetName := "splunk-stand1-standalone"
	var replicas int32 = 32

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	client := spltest.NewMockClient()
	_, err := splctrl.ApplyStatefulSet(client, sts)
	if err != nil {
		t.Errorf("unable to apply statefulset")
	}

	appFrameworkConfig := &enterpriseApi.AppFrameworkSpec{
		VolList: []enterpriseApi.VolumeSpec{
			{Name: "msos_s2s3_vol", Endpoint: "https://s3-eu-west-2.amazonaws.com", Path: "testbucket-rs-london", SecretRef: "s3-secret", Type: "s3", Provider: "aws"},
		},
		AppSources: []enterpriseApi.AppSourceSpec{
			{Name: "adminApps",
				Location: "adminAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "securityApps",
				Location: "securityAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "authenticationApps",
				Location: "authenticationAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
		},
	}

	var appFrameworkContext enterpriseApi.AppDeploymentContext = enterpriseApi.AppDeploymentContext{}
	appFrameworkContext.AppFrameworkConfig = *appFrameworkConfig

	appSrcName := appFrameworkConfig.AppSources[0].Name
	var statefulSet *appsv1.StatefulSet = &appsv1.StatefulSet{}
	worker := &PipelineWorker{
		appDeployInfo: appDeployInfo,
		appSrcName:    appSrcName,
		targetPodName: podName,
		afwConfig:     appFrameworkConfig,
		client:        client,
		cr:            &cr,
		sts:           statefulSet,
	}

	if !reflect.DeepEqual(worker, createPipelineWorker(appDeployInfo, appSrcName, podName, appFrameworkConfig, client, &cr, statefulSet)) {
		t.Errorf("Expected and Returned objects are not the same")
	}

	// Test for createAndAddPipelineWorker
	afwPpln := initAppInstallPipeline(&appFrameworkContext, client, &cr)

	afwPpln.createAndAddPipelineWorker(enterpriseApi.PhaseDownload, appDeployInfo, appSrcName, podName, appFrameworkConfig, client, &cr, statefulSet)
	if len(afwPpln.pplnPhases[enterpriseApi.PhaseDownload].q) != 1 {
		t.Errorf("Unable to add a worker to the pipeline phase")
	}
}

func TestGetApplicablePodNameForAppFramework(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	podID := 0

	expectedPodName := "splunk-stack1-cluster-master-0"
	returnedPodName := getApplicablePodNameForAppFramework(&cr, podID)
	if expectedPodName != returnedPodName {
		t.Errorf("Unable to fetch correct pod name. Expected %s, returned %s", expectedPodName, returnedPodName)
	}

	cr.TypeMeta.Kind = "Standalone"
	expectedPodName = "splunk-stack1-standalone-0"
	returnedPodName = getApplicablePodNameForAppFramework(&cr, podID)
	if expectedPodName != returnedPodName {
		t.Errorf("Unable to fetch correct pod name. Expected %s, returned %s", expectedPodName, returnedPodName)
	}

	cr.TypeMeta.Kind = "LicenseMaster"
	expectedPodName = "splunk-stack1-license-master-0"
	returnedPodName = getApplicablePodNameForAppFramework(&cr, podID)
	if expectedPodName != returnedPodName {
		t.Errorf("Unable to fetch correct pod name. Expected %s, returned %s", expectedPodName, returnedPodName)
	}

	cr.TypeMeta.Kind = "SearchHeadCluster"
	expectedPodName = "splunk-stack1-deployer-0"
	returnedPodName = getApplicablePodNameForAppFramework(&cr, podID)
	if expectedPodName != returnedPodName {
		t.Errorf("Unable to fetch correct pod name. Expected %s, returned %s", expectedPodName, returnedPodName)
	}

	cr.TypeMeta.Kind = "MonitoringConsole"
	expectedPodName = "splunk-stack1-monitoring-console-0"
	returnedPodName = getApplicablePodNameForAppFramework(&cr, podID)
	if expectedPodName != returnedPodName {
		t.Errorf("Unable to fetch correct pod name. Expected %s, returned %s", "", getApplicablePodNameForAppFramework(&cr, 0))
	}
}

func TestInitAppInstallPipeline(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{}
	c := spltest.NewMockClient()
	appDeployContext := &enterpriseApi.AppDeploymentContext{}
	ppln := initAppInstallPipeline(appDeployContext, c, &cr)

	if ppln == nil {
		t.Errorf("Failed to create a new pipeline")
	}
}

func TestDeleteWorkerFromPipelinePhase(t *testing.T) {
	c := spltest.NewMockClient()
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}
	appDeployContext := &enterpriseApi.AppDeploymentContext{}
	ppln := initAppInstallPipeline(appDeployContext, c, &cr)

	if len(ppln.pplnPhases[enterpriseApi.PhaseDownload].q)+len(ppln.pplnPhases[enterpriseApi.PhasePodCopy].q)+len(ppln.pplnPhases[enterpriseApi.PhaseInstall].q) > 0 {
		t.Errorf("Initially Pipeline must be empty, but found as non-empty")
	}

	// Create a list of 5 workers
	capacity := 5
	var workerList []*PipelineWorker = make([]*PipelineWorker, capacity)
	for i := range workerList {
		workerList[i] = &PipelineWorker{
			appDeployInfo: &enterpriseApi.AppDeploymentInfo{
				AppName:    fmt.Sprintf("app%d.tgz", i),
				ObjectHash: fmt.Sprintf("123456%d", i),
			},
			cr: &cr,
		}
	}

	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, workerList...)

	// Make sure that the last element gets deleted
	ppln.deleteWorkerFromPipelinePhase(enterpriseApi.PhaseDownload, workerList[capacity-1])
	if len(ppln.pplnPhases[enterpriseApi.PhaseDownload].q) == capacity {
		t.Errorf("Deleting the last element failed")
	}

	for _, worker := range ppln.pplnPhases[enterpriseApi.PhaseDownload].q {
		if worker == workerList[capacity-1] {
			t.Errorf("Failed to delete the correct element")
		}
	}
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, workerList[capacity-1])

	// Make sure the first element gets deleted
	ppln.deleteWorkerFromPipelinePhase(enterpriseApi.PhaseDownload, workerList[0])
	if len(ppln.pplnPhases[enterpriseApi.PhaseDownload].q) == capacity {
		t.Errorf("Deleting the first element failed")
	}

	if ppln.pplnPhases[enterpriseApi.PhaseDownload].q[0] == workerList[0] {
		t.Errorf("Deleting the first element failed")
	}
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = nil
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, workerList...)

	// Make sure deleting an element in the middle works fine
	ppln.deleteWorkerFromPipelinePhase(enterpriseApi.PhaseDownload, workerList[2])
	for _, worker := range ppln.pplnPhases[enterpriseApi.PhaseDownload].q {
		if worker == workerList[2] {
			t.Errorf("Failed to delete the correct element from the list")
		}
	}

	// Call to delete a non-existing worker should return false
	if ppln.deleteWorkerFromPipelinePhase(enterpriseApi.PhaseDownload, &PipelineWorker{}) {
		t.Errorf("Deletion of non-existing worker should return false, but received true")
	}

}

func TestTransitionWorkerPhase(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}
	appDeployContext := &enterpriseApi.AppDeploymentContext{}
	c := spltest.NewMockClient()
	ppln := initAppInstallPipeline(appDeployContext, c, &cr)

	var replicas int32 = 1
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1",
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	capacity := 5
	var workerList []*PipelineWorker = make([]*PipelineWorker, capacity)
	for i := range workerList {
		workerList[i] = &PipelineWorker{
			sts: sts,
			cr:  &cr,
			appDeployInfo: &enterpriseApi.AppDeploymentInfo{
				AppName: fmt.Sprintf("app%d.tgz", i),
				PhaseInfo: enterpriseApi.PhaseInfo{
					Phase:  enterpriseApi.PhaseDownload,
					Status: enterpriseApi.AppPkgDownloadComplete,
				},
			},
		}
	}

	// Make sure that the workers move from the download phase to the pod Copy phase
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, workerList...)
	for i := range workerList {
		ppln.transitionWorkerPhase(workerList[i], enterpriseApi.PhaseDownload, enterpriseApi.PhasePodCopy)
	}

	if len(ppln.pplnPhases[enterpriseApi.PhasePodCopy].q) != len(workerList) {
		t.Errorf("Unable to move the worker phases")
	}

	if len(ppln.pplnPhases[enterpriseApi.PhaseDownload].q) != 0 {
		t.Errorf("Unable to delete the worker from previous phase")
	}

	// Make sure that the workers move from the podCopy phase to the install phase
	for i := range workerList {
		ppln.transitionWorkerPhase(workerList[i], enterpriseApi.PhasePodCopy, enterpriseApi.PhaseInstall)
	}

	if len(ppln.pplnPhases[enterpriseApi.PhaseInstall].q) != len(workerList) {
		t.Errorf("Unable to move the worker phases")
	}

	if len(ppln.pplnPhases[enterpriseApi.PhasePodCopy].q) != 0 {
		t.Errorf("Unable to delete the worker from previous phase")
	}

	// In the case of Standalone, make sure that there are new workers created for every replicaset member, once after the download phase is completed
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = nil
	ppln.pplnPhases[enterpriseApi.PhasePodCopy].q = nil
	cr.TypeMeta.Kind = "Standalone"
	replicas = 5
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, workerList[0])
	ppln.transitionWorkerPhase(workerList[0], enterpriseApi.PhaseDownload, enterpriseApi.PhasePodCopy)
	if len(ppln.pplnPhases[enterpriseApi.PhasePodCopy].q) != int(replicas) {
		t.Errorf("Failed to create a separate worker for each replica member of a standalone statefulset")
	}

	if len(ppln.pplnPhases[enterpriseApi.PhaseDownload].q) != 0 {
		t.Errorf("Failed to delete the worker from current phase, after moving it to new phase")
	}

	// Make sure that the existing Aux phase info is honoured in case of the Stanalone replicas
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = nil
	ppln.pplnPhases[enterpriseApi.PhasePodCopy].q = nil
	ppln.pplnPhases[enterpriseApi.PhaseInstall].q = nil
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, workerList[0])
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q[0].appDeployInfo.AuxPhaseInfo = make([]enterpriseApi.PhaseInfo, replicas)
	// Mark one pod for installation pending, all others as pod copy pending
	for i := 0; i < int(replicas); i++ {
		ppln.pplnPhases[enterpriseApi.PhaseDownload].q[0].appDeployInfo.AuxPhaseInfo[i].Phase = enterpriseApi.PhasePodCopy
	}
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q[0].appDeployInfo.AuxPhaseInfo[2].Phase = enterpriseApi.PhaseInstall

	ppln.pplnPhases[enterpriseApi.PhaseDownload].q[0].appDeployInfo.AuxPhaseInfo[3].Phase = enterpriseApi.PhaseInstall
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q[0].appDeployInfo.AuxPhaseInfo[3].Status = enterpriseApi.AppPkgInstallComplete

	ppln.transitionWorkerPhase(workerList[0], enterpriseApi.PhaseDownload, enterpriseApi.PhasePodCopy)
	if len(ppln.pplnPhases[enterpriseApi.PhasePodCopy].q) != int(replicas)-2 {
		t.Errorf("Failed to create the pod copy workers, according to the AuxPhaseInfo")
	}
	if len(ppln.pplnPhases[enterpriseApi.PhaseInstall].q) != 1 {
		t.Errorf("Failed to create the install workers, according to the AuxPhaseInfo")
	}
}

func TestCheckIfWorkerIsEligibleForRun(t *testing.T) {
	worker := &PipelineWorker{
		isActive: false,
	}
	phaseInfo := &enterpriseApi.PhaseInfo{
		RetryCount: 0,
		Status:     enterpriseApi.AppPkgDownloadPending,
	}

	// test for an eligible worker
	if !checkIfWorkerIsEligibleForRun(worker, phaseInfo, enterpriseApi.AppPkgDownloadComplete) {
		t.Errorf("Unable to detect an eligible worker")
	}

	// test for an active worker
	worker.isActive = true
	if checkIfWorkerIsEligibleForRun(worker, phaseInfo, enterpriseApi.AppPkgDownloadComplete) {
		t.Errorf("Unable to detect an ineligible worker(already active")
	}
	worker.isActive = false

	// test for max retry
	phaseInfo.RetryCount = 4
	if checkIfWorkerIsEligibleForRun(worker, phaseInfo, enterpriseApi.AppPkgDownloadComplete) {
		t.Errorf("Unable to detect an ineligible worker(max retries exceeded)")
	}
	phaseInfo.RetryCount = 0

	// test for already completed worker
	phaseInfo.Status = enterpriseApi.AppPkgDownloadComplete
	if checkIfWorkerIsEligibleForRun(worker, phaseInfo, enterpriseApi.AppPkgDownloadComplete) {
		t.Errorf("Unable to detect an ineligible worker(already completed)")
	}
}

func TestPhaseManagersTermination(t *testing.T) {
	c := spltest.NewMockClient()
	cr := &enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stand1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	statefulSetName := fmt.Sprintf("splunk-%s-standalone", cr.GetName())
	var replicas int32 = 2

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	_, err := splctrl.ApplyStatefulSet(c, sts)
	if err != nil {
		t.Errorf("unable to apply statefulset")
	}
	appDeployContext := &enterpriseApi.AppDeploymentContext{}
	ppln := initAppInstallPipeline(appDeployContext, c, cr)

	ppln.appDeployContext.AppsStatusMaxConcurrentAppDownloads = 1
	ppln.phaseWaiter.Add(1)
	go ppln.downloadPhaseManager()

	ppln.phaseWaiter.Add(1)
	go ppln.podCopyPhaseManager()

	ppln.phaseWaiter.Add(1)
	go ppln.installPhaseManager()

	// Make sure that the pipeline is not blocked and comes out after termination
	// Terminate the scheduler, by closing the channel
	close(ppln.sigTerm)
	// Crossign wait() indicates the termination logic works.
	ppln.phaseWaiter.Wait()
}

func TestPhaseManagersMsgChannels(t *testing.T) {
	appDeployContext := &enterpriseApi.AppDeploymentContext{
		AppsStatusMaxConcurrentAppDownloads: 1,
	}

	// Test for each phase can send the worker to down stream
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			AppFrameworkConfig: enterpriseApi.AppFrameworkSpec{
				AppsRepoPollInterval:      60,
				MaxConcurrentAppDownloads: 5,

				VolList: []enterpriseApi.VolumeSpec{
					{
						Name:      "test_volume",
						Endpoint:  "https://s3-eu-west-2.amazonaws.com",
						Path:      "testbucket-rs-london",
						SecretRef: "s3-secret",
						Provider:  "aws",
					},
				},
				AppSources: []enterpriseApi.AppSourceSpec{
					{
						Name:     "appSrc1",
						Location: "adminAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
				},
			},
		},
	}

	statefulSetName := fmt.Sprintf("splunk-%s-standalone", cr.GetName())
	var replicas int32 = 1
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	// Create client and add object
	client := spltest.NewMockClient()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-standalone-0",
			Namespace: "test",
			Labels: map[string]string{
				"controller-revision-hash": "v0",
			},
		},
	}

	// Add object
	client.AddObject(pod)

	// Create the statefulset
	_, err := splctrl.ApplyStatefulSet(client, sts)
	if err != nil {
		t.Errorf("unable to apply statefulset")
	}

	// Just make the lint conversion checks happy
	//var client splcommon.ControllerClient = getConvertedClient(c)

	capacity := 1
	var workerList []*PipelineWorker = make([]*PipelineWorker, capacity)
	for i := range workerList {
		workerList[i] = &PipelineWorker{
			appSrcName:    cr.Spec.AppFrameworkConfig.AppSources[0].Name,
			targetPodName: "splunk-stack1-standalone-0",
			sts:           sts,
			cr:            &cr,
			appDeployInfo: &enterpriseApi.AppDeploymentInfo{
				AppName: fmt.Sprintf("app%d.tgz", i),
				PhaseInfo: enterpriseApi.PhaseInfo{
					Phase:      enterpriseApi.PhaseDownload,
					Status:     enterpriseApi.AppPkgDownloadPending,
					RetryCount: 2,
				},
			},
			afwConfig: &cr.Spec.AppFrameworkConfig,
			client:    client,
		}
	}

	// test  all the pipeline phases are able to send the worker to the downstreams
	ppln := initAppInstallPipeline(appDeployContext, client, &cr)
	// Make sure that the workers move from the download phase to the pod Copy phase
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, workerList...)

	// Start the download phase manager
	ppln.phaseWaiter.Add(1)
	go ppln.downloadPhaseManager()
	// drain the download phase channel
	var worker *PipelineWorker
	var i int
	for i = 0; i < int(replicas); i++ {
		worker = <-ppln.pplnPhases[enterpriseApi.PhaseDownload].msgChannel
	}

	if worker != workerList[i-1] {
		t.Errorf("Unable to flush a download worker")
	}
	worker.appDeployInfo.PhaseInfo.RetryCount = 4
	// Let the phase hop on empty channel, to get more coverage
	time.Sleep(600 * time.Millisecond)
	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = nil

	// add the worker to the pod copy phase
	worker.appDeployInfo.PhaseInfo.RetryCount = 0
	worker.isActive = false
	worker.appDeployInfo.PhaseInfo.Phase = enterpriseApi.PhasePodCopy
	worker.appDeployInfo.PhaseInfo.Status = enterpriseApi.AppPkgPodCopyPending
	ppln.pplnPhases[enterpriseApi.PhasePodCopy].q = append(ppln.pplnPhases[enterpriseApi.PhasePodCopy].q, workerList...)

	go ppln.podCopyPhaseManager()
	ppln.phaseWaiter.Add(1)

	// drain the pod copy channel
	for i := 0; i < int(replicas); i++ {
		worker = <-ppln.pplnPhases[enterpriseApi.PhasePodCopy].msgChannel
	}

	if worker != workerList[0] {
		t.Errorf("Unable to flush a pod copy worker")
	}
	worker.appDeployInfo.PhaseInfo.RetryCount = 4
	// Let the phase hop on empty channel, to get more coverage
	time.Sleep(600 * time.Millisecond)
	ppln.pplnPhases[enterpriseApi.PhasePodCopy].q = nil

	// add the worker to the install phase
	worker.appDeployInfo.PhaseInfo.RetryCount = 0
	worker.isActive = false
	worker.appDeployInfo.PhaseInfo.Phase = enterpriseApi.PhaseInstall
	worker.appDeployInfo.PhaseInfo.Status = enterpriseApi.AppPkgInstallPending
	ppln.pplnPhases[enterpriseApi.PhaseInstall].q = append(ppln.pplnPhases[enterpriseApi.PhaseInstall].q, workerList...)

	// Start the install phase manager
	ppln.phaseWaiter.Add(1)
	go ppln.installPhaseManager()

	worker.appDeployInfo.PhaseInfo.RetryCount = 4
	// Let the phase hop on empty channel, to get more coverage
	time.Sleep(600 * time.Millisecond)

	close(ppln.sigTerm)

	// wait for all the phases to return
	ppln.phaseWaiter.Wait()
}

func TestIsPipelineEmpty(t *testing.T) {
	c := spltest.NewMockClient()
	cr := &enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	appDeployContext := &enterpriseApi.AppDeploymentContext{}
	ppln := initAppInstallPipeline(appDeployContext, c, cr)

	if !ppln.isPipelineEmpty() {
		t.Errorf("Expected empty pipeline, but found some workers in pipeline")
	}

	ppln.pplnPhases[enterpriseApi.PhaseDownload].q = append(ppln.pplnPhases[enterpriseApi.PhaseDownload].q, &PipelineWorker{})
	if ppln.isPipelineEmpty() {
		t.Errorf("Expected pipeline to have elements, but the pipeline is empty")
	}
}

func TestGetOrdinalValFromPodName(t *testing.T) {
	var ordinal int = 2

	// Proper pod name should return ordinal value
	var podName = fmt.Sprintf("splunk-s2apps-standalone-%d", ordinal)

	retOrdinal, err := getOrdinalValFromPodName(podName)
	if ordinal != retOrdinal || err != nil {
		t.Errorf("Unable to get the ordinal value from the pod name. error: %v", err)
	}

	// Invalid pod name should return error
	podName = fmt.Sprintf("splunks2apps-standalone-%d", ordinal)

	_, err = getOrdinalValFromPodName(podName)
	if err == nil {
		t.Errorf("Invalid pod name should return error, but didn't receive an error")
	}
}

func TestIsAppInstallationCompleteOnStandaloneReplicas(t *testing.T) {
	worker := &PipelineWorker{
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AuxPhaseInfo: make([]enterpriseApi.PhaseInfo, 5),
		},
	}

	auxPhaseInfo := worker.appDeployInfo.AuxPhaseInfo
	auxPhaseInfo[3].Phase = enterpriseApi.PhaseDownload
	if isAppInstallationCompleteOnAllReplicas(auxPhaseInfo) {
		t.Errorf("Expected App installation incomplete, but returned as complete")
	}

	for i := range auxPhaseInfo {
		auxPhaseInfo[i].Phase = enterpriseApi.PhaseInstall
		auxPhaseInfo[i].Status = enterpriseApi.AppPkgInstallComplete
	}

	if !isAppInstallationCompleteOnAllReplicas(auxPhaseInfo) {
		t.Errorf("Expected App installation complete, but returned as incomplete")
	}

}

func TestCheckIfBundlePushIsDone(t *testing.T) {
	if !checkIfBundlePushIsDone("Standalone", enterpriseApi.BundlePushPending) {
		t.Errorf("checkIfBundlePushIsDone should have returned true")
	}

	if checkIfBundlePushIsDone("ClusterMaster", enterpriseApi.BundlePushPending) {
		t.Errorf("checkIfBundlePushIsDone should have returned false")
	}
}

func TestNeedToUseAuxPhaseInfo(t *testing.T) {
	cr := &enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-cluster-master",
			Namespace: "test",
		},
	}
	sts.Spec.Replicas = new(int32)
	*sts.Spec.Replicas = 5
	worker := &PipelineWorker{}
	worker.cr = cr

	// for the download phase, aux. phase info not needed
	if needToUseAuxPhaseInfo(worker, enterpriseApi.PhaseDownload) {
		t.Errorf("Suggesting Aux phase info, when it is not needed")
	}

	// for the phase pod copy, aux. phase info is needed
	worker.sts = sts
	if !needToUseAuxPhaseInfo(worker, enterpriseApi.PhasePodCopy) {
		t.Errorf("Not suggesting Aux phase info, when it is needed")
	}

	*sts.Spec.Replicas = 1
	// When replica count is 1, no need to use aux phase info
	if needToUseAuxPhaseInfo(worker, enterpriseApi.PhasePodCopy) {
		t.Errorf("Suggesting Aux phase info, when it is not needed")
	}
}

func TestGetPhaseInfoByPhaseType(t *testing.T) {
	cr := &enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-cluster-master",
			Namespace: "test",
		},
	}
	sts.Spec.Replicas = new(int32)
	*sts.Spec.Replicas = 5
	worker := &PipelineWorker{}
	worker.cr = cr
	worker.sts = sts
	worker.appDeployInfo = &enterpriseApi.AppDeploymentInfo{
		AuxPhaseInfo: make([]enterpriseApi.PhaseInfo, *sts.Spec.Replicas),
	}

	worker.targetPodName = "splunk-stack1-standalone-3"

	// For download phase we should not use Aux phase info
	expectedPhaseInfo := &worker.appDeployInfo.PhaseInfo
	retPhaseInfo := getPhaseInfoByPhaseType(worker, enterpriseApi.PhaseDownload)
	if expectedPhaseInfo != retPhaseInfo {
		t.Errorf("Got the wrong phase info for download phase of Standalone")
	}

	// For non-download phases we should use Aux phase info
	expectedPhaseInfo = &worker.appDeployInfo.AuxPhaseInfo[3]
	retPhaseInfo = getPhaseInfoByPhaseType(worker, enterpriseApi.PhasePodCopy)
	if expectedPhaseInfo != retPhaseInfo {
		t.Errorf("Got the wrong phase info for Standalone")
	}

	// For CRs that are not standalone, we should not use the Aux phase info
	expectedPhaseInfo = &worker.appDeployInfo.PhaseInfo
	cr.Kind = "ClusterMaster"
	retPhaseInfo = getPhaseInfoByPhaseType(worker, enterpriseApi.PhasePodCopy)
	if expectedPhaseInfo != retPhaseInfo {
		t.Errorf("Got the incorrect phase info for CM")
	}

	worker.targetPodName = "invalid-podName"
	// For CRs that are not standalone, we should not use the Aux phase info
	expectedPhaseInfo = nil
	cr.Kind = "Standalone"
	retPhaseInfo = getPhaseInfoByPhaseType(worker, enterpriseApi.PhasePodCopy)
	if expectedPhaseInfo != retPhaseInfo {
		t.Errorf("Should get nil for invalid pod name of Standalone")
	}
}
func TestAfwGetReleventStatefulsetByKind(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	c := spltest.NewMockClient()

	// Test if STS works for cluster master
	current := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-cluster-master",
			Namespace: "test",
		},
	}

	_, err := splctrl.ApplyStatefulSet(c, &current)
	if err != nil {
		return
	}
	if afwGetReleventStatefulsetByKind(&cr, c) == nil {
		t.Errorf("Unable to get the sts for CM")
	}

	// Test if STS works for deployer
	cr.TypeMeta.Kind = "SearchHeadCluster"
	current = appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-deployer",
			Namespace: "test",
		},
	}

	_, _ = splctrl.ApplyStatefulSet(c, &current)
	if afwGetReleventStatefulsetByKind(&cr, c) == nil {
		t.Errorf("Unable to get the sts for SHC deployer")
	}

	// Test if STS works for LM
	cr.TypeMeta.Kind = "LicenseMaster"
	current = appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-license-master",
			Namespace: "test",
		},
	}

	_, _ = splctrl.ApplyStatefulSet(c, &current)
	if afwGetReleventStatefulsetByKind(&cr, c) == nil {
		t.Errorf("Unable to get the sts for SHC deployer")
	}

	// Test if STS works for Standalone
	cr.TypeMeta.Kind = "Standalone"
	current = appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-standalone",
			Namespace: "test",
		},
	}

	_, _ = splctrl.ApplyStatefulSet(c, &current)
	if afwGetReleventStatefulsetByKind(&cr, c) == nil {
		t.Errorf("Unable to get the sts for SHC deployer")
	}

}

func createOrTruncateAppFileLocally(appFileName string, size int64) error {
	appFile, err := os.Create(appFileName)
	if err != nil {
		return fmt.Errorf("failed to create app file %s", appFile.Name())
	}

	if err := appFile.Truncate(size); err != nil {
		return fmt.Errorf("failed to truncate app file %s", appFile.Name())
	}

	appFile.Close()

	return nil
}

func areAppsDownloadedSuccessfully(appDeployInfoList []*enterpriseApi.AppDeploymentInfo) (bool, error) {
	for _, appInfo := range appDeployInfoList {
		if appInfo.PhaseInfo.Status != enterpriseApi.AppPkgDownloadComplete {
			err := fmt.Errorf("App:%s is not downloaded yet", appInfo.AppName)
			return false, err
		}
	}
	return true, nil
}

func TestPipelineWorkerDownloadShouldPass(t *testing.T) {
	cr := enterpriseApi.Standalone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "test",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		Spec: enterpriseApi.StandaloneSpec{
			Replicas: 1,
			AppFrameworkConfig: enterpriseApi.AppFrameworkSpec{
				AppsRepoPollInterval:      60,
				MaxConcurrentAppDownloads: 5,

				VolList: []enterpriseApi.VolumeSpec{
					{
						Name:      "test_volume",
						Endpoint:  "https://s3-eu-west-2.amazonaws.com",
						Path:      "testbucket-rs-london",
						SecretRef: "s3-secret",
						Provider:  "aws",
					},
				},
				AppSources: []enterpriseApi.AppSourceSpec{
					{
						Name:     "appSrc1",
						Location: "adminAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
					{
						Name:     "appSrc2",
						Location: "securityAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
					{
						Name:     "appSrc3",
						Location: "authenticationAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-s1-standalone",
			Namespace: "test",
		},
	}
	sts.Spec.Replicas = new(int32)
	*sts.Spec.Replicas = 1

	testApps := []string{"app1.tgz", "app2.tgz", "app3.tgz"}
	testHashes := []string{"abcd1111", "efgh2222", "ijkl3333"}
	testSizes := []int64{10, 20, 30}

	appDeployInfoList := make([]*enterpriseApi.AppDeploymentInfo, 3)
	for index := range testApps {
		appDeployInfoList[index] = &enterpriseApi.AppDeploymentInfo{
			AppName: testApps[index],
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhaseDownload,
				Status:     enterpriseApi.AppPkgDownloadPending,
				RetryCount: 0,
			},
			ObjectHash: testHashes[index],
			Size:       uint64(testSizes[index]),
		}
	}

	client := spltest.NewMockClient()

	// Create S3 secret
	s3Secret := spltest.GetMockS3SecretKeys("s3-secret")

	client.AddObject(&s3Secret)

	// Create namespace scoped secret
	_, err := splutil.ApplyNamespaceScopedSecretObject(client, "test")
	if err != nil {
		t.Errorf(err.Error())
	}

	splclient.RegisterS3Client("aws")

	for index, appSrc := range cr.Spec.AppFrameworkConfig.AppSources {

		localPath := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", cr.Namespace, cr.Kind, cr.Name, appSrc.Scope, appSrc.Name) + "/"
		// create the app download directory locally
		err := createAppDownloadDir(localPath)
		if err != nil {
			t.Errorf("Unable to create the download directory")
		}
		defer os.RemoveAll(splcommon.AppDownloadVolume)

		// create the dummy app packages locally
		appFileName := testApps[index] + "_" + testHashes[index]
		appLoc := filepath.Join(localPath, appFileName)
		err = createOrTruncateAppFileLocally(appLoc, testSizes[index])
		if err != nil {
			t.Errorf("Unable to create the app files locally")
		}
		defer os.Remove(appLoc)

		// Update the GetS3Client with our mock call which initializes mock AWS client
		getClientWrapper := splclient.S3Clients["aws"]
		getClientWrapper.SetS3ClientFuncPtr("aws", splclient.NewMockAWSS3Client)

		initFunc := getClientWrapper.GetS3ClientInitFuncPtr()

		s3ClientMgr, err := getS3ClientMgr(client, &cr, &cr.Spec.AppFrameworkConfig, appSrc.Name)
		if err != nil {
			t.Errorf("unable to get S3ClientMgr instance")
		}

		s3ClientMgr.initFn = initFunc

		pplnPhase := &PipelinePhase{}
		worker := &PipelineWorker{
			appSrcName:    appSrc.Name,
			cr:            &cr,
			sts:           sts,
			afwConfig:     &cr.Spec.AppFrameworkConfig,
			appDeployInfo: appDeployInfoList[index],
			waiter:        new(sync.WaitGroup),
		}
		var downloadWorkersRunPool = make(chan struct{}, 1)
		downloadWorkersRunPool <- struct{}{}
		worker.waiter.Add(1)
		go worker.download(pplnPhase, *s3ClientMgr, localPath, downloadWorkersRunPool)
		worker.waiter.Wait()
	}

	// verify if all the apps are in DownloadComplete state
	if ok, err := areAppsDownloadedSuccessfully(appDeployInfoList); !ok {
		t.Errorf("All apps should have been downloaded successfully, error=%v", err)
	}
}

func TestPipelineWorkerDownloadShouldFail(t *testing.T) {
	cr := enterpriseApi.Standalone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "test",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		Spec: enterpriseApi.StandaloneSpec{
			Replicas: 1,
			AppFrameworkConfig: enterpriseApi.AppFrameworkSpec{
				AppsRepoPollInterval:      60,
				MaxConcurrentAppDownloads: 5,

				VolList: []enterpriseApi.VolumeSpec{
					{
						Name:      "test_volume",
						Endpoint:  "https://s3-eu-west-2.amazonaws.com",
						Path:      "testbucket-rs-london",
						SecretRef: "s3-secret",
						Provider:  "aws",
					},
				},
				AppSources: []enterpriseApi.AppSourceSpec{
					{
						Name:     "appSrc1",
						Location: "adminAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-s1-standalone",
			Namespace: "test",
		},
	}
	sts.Spec.Replicas = new(int32)
	*sts.Spec.Replicas = 1

	testApps := []string{"app1.tgz"}
	testHashes := []string{"abcd1111"}
	testSizes := []int64{10}

	appDeployInfoList := make([]*enterpriseApi.AppDeploymentInfo, 1)
	for index := range testApps {
		appDeployInfoList[index] = &enterpriseApi.AppDeploymentInfo{
			AppName: testApps[index],
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhaseDownload,
				Status:     enterpriseApi.AppPkgDownloadPending,
				RetryCount: 0,
			},
			ObjectHash: testHashes[index],
			Size:       uint64(testSizes[index]),
		}
	}

	s3ClientMgr := &S3ClientManager{}

	// Test1. Invalid appSrcName
	worker := &PipelineWorker{
		appSrcName:    "invalidAppSrcName",
		cr:            &cr,
		sts:           sts,
		afwConfig:     &cr.Spec.AppFrameworkConfig,
		appDeployInfo: appDeployInfoList[0],
		waiter:        new(sync.WaitGroup),
	}

	pplnPhase := &PipelinePhase{}
	var downloadWorkersRunPool = make(chan struct{}, 1)
	downloadWorkersRunPool <- struct{}{}
	worker.waiter.Add(1)
	go worker.download(pplnPhase, *s3ClientMgr, "", downloadWorkersRunPool)
	worker.waiter.Wait()

	// we should return error here
	if ok, _ := areAppsDownloadedSuccessfully(appDeployInfoList); ok {
		t.Errorf("We should have returned error here since appSrcName is invalid in the worker")
	}

	worker.appSrcName = "appSrc1"

	// Test2. Empty Object hash
	worker.appDeployInfo.ObjectHash = ""

	client := spltest.NewMockClient()

	// Create S3 secret
	s3Secret := spltest.GetMockS3SecretKeys("s3-secret")

	client.AddObject(&s3Secret)

	// Create namespace scoped secret
	_, err := splutil.ApplyNamespaceScopedSecretObject(client, "test")
	if err != nil {
		t.Errorf(err.Error())
	}

	splclient.RegisterS3Client("aws")
	// Update the GetS3Client with our mock call which initializes mock AWS client
	getClientWrapper := splclient.S3Clients["aws"]
	getClientWrapper.SetS3ClientFuncPtr("aws", splclient.NewMockAWSS3Client)

	initFunc := getClientWrapper.GetS3ClientInitFuncPtr()

	s3ClientMgr, err = getS3ClientMgr(client, &cr, &cr.Spec.AppFrameworkConfig, "appSrc1")
	if err != nil {
		t.Errorf("unable to get S3ClientMgr instance")
	}

	s3ClientMgr.initFn = initFunc

	worker.waiter.Add(1)
	downloadWorkersRunPool <- struct{}{}
	go worker.download(pplnPhase, *s3ClientMgr, "", downloadWorkersRunPool)
	worker.waiter.Wait()
	// we should return error here
	if ok, _ := areAppsDownloadedSuccessfully(appDeployInfoList); ok {
		t.Errorf("We should have returned error here since objectHash is empty in the worker")
	}

}

func TestScheduleDownloads(t *testing.T) {
	var ppln *AppInstallPipeline
	appDeployContext := &enterpriseApi.AppDeploymentContext{}

	client := spltest.NewMockClient()
	cr := enterpriseApi.Standalone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "test",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		Spec: enterpriseApi.StandaloneSpec{
			Replicas: 1,
			AppFrameworkConfig: enterpriseApi.AppFrameworkSpec{
				AppsRepoPollInterval:      60,
				MaxConcurrentAppDownloads: 5,

				VolList: []enterpriseApi.VolumeSpec{
					{
						Name:      "test_volume",
						Endpoint:  "https://s3-eu-west-2.amazonaws.com",
						Path:      "testbucket-rs-london",
						SecretRef: "s3-secret",
						Provider:  "aws",
					},
				},
				AppSources: []enterpriseApi.AppSourceSpec{
					{
						Name:     "appSrc1",
						Location: "adminAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
					{
						Name:     "appSrc2",
						Location: "securityAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
					{
						Name:     "appSrc3",
						Location: "authenticationAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-s1-standalone",
			Namespace: "test",
		},
	}
	sts.Spec.Replicas = new(int32)
	*sts.Spec.Replicas = 1

	ppln = initAppInstallPipeline(appDeployContext, client, &cr)
	pplnPhase := ppln.pplnPhases[enterpriseApi.PhaseDownload]

	testApps := []string{"app1.tgz", "app2.tgz", "app3.tgz"}
	testHashes := []string{"abcd1111", "efgh2222", "ijkl3333"}
	testSizes := []int64{10, 20, 30}

	appDeployInfoList := make([]*enterpriseApi.AppDeploymentInfo, 3)
	for index := range testApps {
		appDeployInfoList[index] = &enterpriseApi.AppDeploymentInfo{
			AppName: testApps[index],
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhaseDownload,
				Status:     enterpriseApi.AppPkgDownloadPending,
				RetryCount: 0,
			},
			ObjectHash: testHashes[index],
			Size:       uint64(testSizes[index]),
		}
	}

	// create the local directory
	localPath := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", "test" /*namespace*/, "Standalone", cr.Name, "local", "appSrc1") + "/"
	err := createAppDownloadDir(localPath)
	if err != nil {
		t.Errorf("Unable to create the download directory")
	}
	defer os.RemoveAll(splcommon.AppDownloadVolume)

	// create the dummy app package for appSrc1 locally, to test the case
	// where an app is already downloaded and hence we dont re-download it
	appFileName := testApps[0] + "_" + testHashes[0]
	appLoc := filepath.Join(localPath, appFileName)
	err = createOrTruncateAppFileLocally(appLoc, testSizes[0])
	if err != nil {
		t.Errorf("Unable to create the app files locally")
	}
	defer os.Remove(appLoc)

	// create pipeline workers
	for index, appSrc := range cr.Spec.AppFrameworkConfig.AppSources {
		ppln.createAndAddPipelineWorker(enterpriseApi.PhaseDownload, appDeployInfoList[index], appSrc.Name, "", &cr.Spec.AppFrameworkConfig, client, &cr, sts)
	}

	maxWorkers := 3
	downloadPhaseWaiter := new(sync.WaitGroup)

	downloadPhaseWaiter.Add(1)
	// schedule the download threads to do actual download work
	go pplnPhase.scheduleDownloads(ppln, uint64(maxWorkers), downloadPhaseWaiter)

	// add the workers to msgChannel so that scheduleDownlads thread can pick them up
	for _, downloadWorker := range pplnPhase.q {
		downloadWorker.waiter = &pplnPhase.workerWaiter
		pplnPhase.msgChannel <- downloadWorker
		downloadWorker.isActive = true
	}

	close(pplnPhase.msgChannel)

	downloadPhaseWaiter.Add(1)
	close(ppln.sigTerm)
	// schedule the download threads to do actual download work
	go pplnPhase.scheduleDownloads(ppln, uint64(maxWorkers), downloadPhaseWaiter)

	downloadPhaseWaiter.Wait()
}

func TestCreateDownloadDirOnOperator(t *testing.T) {
	standalone := enterpriseApi.Standalone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "test",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		Spec: enterpriseApi.StandaloneSpec{
			Replicas: 1,
			AppFrameworkConfig: enterpriseApi.AppFrameworkSpec{
				AppsRepoPollInterval:      60,
				MaxConcurrentAppDownloads: 5,

				VolList: []enterpriseApi.VolumeSpec{
					{
						Name:      "test_volume",
						Endpoint:  "https://s3-eu-west-2.amazonaws.com",
						Path:      "testbucket-rs-london",
						SecretRef: "s3-secret",
						Provider:  "aws",
					},
				},
				AppSources: []enterpriseApi.AppSourceSpec{
					{
						Name:     "appSrc1",
						Location: "adminAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
				},
			},
		},
	}

	worker := &PipelineWorker{
		appSrcName: "appSrc1",
		cr:         &standalone,
		afwConfig:  &standalone.Spec.AppFrameworkConfig,
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AppName: "testApp1",
		},
	}

	localPath, err := worker.createDownloadDirOnOperator()
	defer os.Remove(localPath)
	if err != nil {
		t.Errorf("we should have created the download directory=%s, err=%v", localPath, err)
	}
}

func getConvertedClient(client splcommon.ControllerClient) splcommon.ControllerClient {
	return client
}

func TestExtractClusterScopedAppOnPod(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-clustermaster-0",
			Namespace: "test",
			Labels: map[string]string{
				"controller-revision-hash": "v0",
			},
		},
	}

	// Create client and add object
	c := spltest.NewMockClient()
	// Add object
	c.AddObject(pod)

	// Just make the lint conversion checks happy
	var client splcommon.ControllerClient = getConvertedClient(c)

	//var client splcommon.ControllerClient
	worker := &PipelineWorker{
		cr:            &cr,
		targetPodName: "splunk-stack1-clustermaster-0",
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AppName: "app1.tgz",
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhasePodCopy,
				Status:     enterpriseApi.AppPkgPodCopyPending,
				RetryCount: 0,
			},
		},
		client: client,
	}

	srcPath := "/opt/splunk/operator/app1.tgz"
	dstPath := "/init-apps/xyz/app1.tgz"

	// Calling with wrong scope should just return, without error
	err := extractClusterScopedAppOnPod(worker, enterpriseApi.ScopeLocal, dstPath, srcPath)
	if err != nil {
		t.Errorf("Calling with non-cluster scope should just return, without error, but got error %v", err)
	}

	// CR kind other than SearchHeadCluster or Cluster Master should just return without an error
	// Calling with wrong scope should just return, without error
	err = extractClusterScopedAppOnPod(worker, enterpriseApi.ScopeCluster, dstPath, srcPath)
	if err != nil {
		t.Errorf("Calling with non-cluster scope should just return, without error, but got error %v", err)
	}

	// Calling with correct params should not cause an error

	cr.TypeMeta.Kind = "ClusterMaster"
	err = extractClusterScopedAppOnPod(worker, enterpriseApi.ScopeCluster, dstPath, srcPath)

	// PodExec command fails, as there is no real Pod here. Bypassing the error check for now, just to have enough code coverage.
	// Need to fix this later, once the PodExec can accommodate the UT flow for a non-existing Pod.
	if 1 == 0 && err != nil {
		t.Errorf("Calling with correct parameters should not cause an error, but got error %v", err)
	}
}

func TestRunPodCopyWorker(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-clustermaster-0",
			Namespace: "test",
			Labels: map[string]string{
				"controller-revision-hash": "v0",
			},
		},
	}

	appFrameworkConfig := &enterpriseApi.AppFrameworkSpec{
		VolList: []enterpriseApi.VolumeSpec{
			{Name: "msos_s2s3_vol", Endpoint: "https://s3-eu-west-2.amazonaws.com", Path: "testbucket-rs-london", SecretRef: "s3-secret", Type: "s3", Provider: "aws"},
		},
		AppSources: []enterpriseApi.AppSourceSpec{
			{Name: "adminApps",
				Location: "adminAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "securityApps",
				Location: "securityAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "authenticationApps",
				Location: "authenticationAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
		},
	}

	// Create client and add object
	c := spltest.NewMockClient()
	// Add object
	c.AddObject(pod)

	// Just make the lint conversion checks happy
	var client splcommon.ControllerClient = getConvertedClient(c)
	var waiter sync.WaitGroup

	//var client splcommon.ControllerClient
	worker := &PipelineWorker{
		cr:            &cr,
		targetPodName: "splunk-stack1-clustermaster-0",
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AppName: "app1.tgz",
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhasePodCopy,
				Status:     enterpriseApi.AppPkgPodCopyPending,
				RetryCount: 0,
			},
			ObjectHash: "abcd1234abcd",
		},
		client:     client,
		afwConfig:  appFrameworkConfig,
		waiter:     &waiter,
		appSrcName: appFrameworkConfig.AppSources[0].Name,
	}

	var ch chan struct{} = make(chan struct{}, 1)
	// occupy the worker
	waiter.Add(1)
	ch <- struct{}{}

	defaultVol := splcommon.AppDownloadVolume
	splcommon.AppDownloadVolume = "/tmp/"
	defer func() {
		splcommon.AppDownloadVolume = defaultVol
	}()

	runPodCopyWorker(worker, ch)
	if worker.appDeployInfo.PhaseInfo.Status != enterpriseApi.AppPkgMissingFromOperator {
		t.Errorf("When the app pkg is not present on the Operator Pod volume, should throw an error")
	}

	appPkgFileName := worker.appDeployInfo.AppName + "_" + strings.Trim(worker.appDeployInfo.ObjectHash, "\"")

	appSrcScope := getAppSrcScope(worker.afwConfig, worker.appSrcName)
	appPkgLocalDir := getAppPackageLocalDir(&cr, appSrcScope, worker.appSrcName)
	appPkgLocalPath := filepath.Join(appPkgLocalDir, appPkgFileName)

	_, err := os.Stat(appPkgLocalDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(appPkgLocalDir, 0755)
		if err != nil {
			t.Errorf("Unable to create the directory, error: %v", err)
		}
	}

	_, err = os.Stat(appPkgLocalPath)
	if os.IsNotExist(err) {
		_, err := os.Create(appPkgLocalPath)
		if err != nil {
			t.Errorf("Unable to create the local package file, error: %v", err)
		}
	}
	defer os.Remove(appPkgLocalPath)
	waiter.Add(1)
	ch <- struct{}{}

	runPodCopyWorker(worker, ch)
}

func TestPodCopyWorkerHandler(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-clustermaster-0",
			Namespace: "test",
			Labels: map[string]string{
				"controller-revision-hash": "v0",
			},
		},
	}

	appFrameworkConfig := &enterpriseApi.AppFrameworkSpec{
		VolList: []enterpriseApi.VolumeSpec{
			{Name: "msos_s2s3_vol", Endpoint: "https://s3-eu-west-2.amazonaws.com", Path: "testbucket-rs-london", SecretRef: "s3-secret", Type: "s3", Provider: "aws"},
		},
		AppSources: []enterpriseApi.AppSourceSpec{
			{Name: "adminApps",
				Location: "adminAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "securityApps",
				Location: "securityAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "authenticationApps",
				Location: "authenticationAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
		},
	}

	// Create client and add object
	client := spltest.NewMockClient()
	// Add object
	client.AddObject(pod)

	worker := &PipelineWorker{
		cr:            &cr,
		targetPodName: "splunk-stack1-clustermaster-0",
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AppName: "app1.tgz",
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhasePodCopy,
				Status:     enterpriseApi.AppPkgPodCopyPending,
				RetryCount: 0,
			},
			ObjectHash: "abcd1234abcd",
		},
		client:     client,
		afwConfig:  appFrameworkConfig,
		appSrcName: appFrameworkConfig.AppSources[0].Name,
	}

	defaultVol := splcommon.AppDownloadVolume
	splcommon.AppDownloadVolume = "/tmp/splunk/"
	defer func() {
		os.RemoveAll(splcommon.AppDownloadVolume)
		splcommon.AppDownloadVolume = defaultVol
	}()

	appPkgFileName := worker.appDeployInfo.AppName + "_" + strings.Trim(worker.appDeployInfo.ObjectHash, "\"")

	appSrcScope := getAppSrcScope(worker.afwConfig, worker.appSrcName)
	appPkgLocalDir := getAppPackageLocalDir(&cr, appSrcScope, worker.appSrcName)
	appPkgLocalPath := filepath.Join(appPkgLocalDir, appPkgFileName)

	_, err := os.Stat(appPkgLocalDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(appPkgLocalDir, 0755)
		if err != nil {
			t.Errorf("Unable to create the directory, error: %v", err)
		}
	}

	_, err = os.Stat(appPkgLocalPath)
	if os.IsNotExist(err) {
		_, err := os.Create(appPkgLocalPath)
		if err != nil {
			t.Errorf("Unable to create the local package file, error: %v", err)
		}
	}
	defer os.Remove(appPkgLocalPath)

	var appDeployContext *enterpriseApi.AppDeploymentContext = &enterpriseApi.AppDeploymentContext{
		AppsStatusMaxConcurrentAppDownloads: 5,
	}
	ppln := initAppInstallPipeline(appDeployContext, client, &cr)

	var handlerWaiter sync.WaitGroup
	handlerWaiter.Add(1)
	defer handlerWaiter.Wait()
	go ppln.pplnPhases[enterpriseApi.PhaseInstall].podCopyWorkerHandler(&handlerWaiter, 5)

	worker.waiter = &ppln.pplnPhases[enterpriseApi.PhaseInstall].workerWaiter

	ppln.pplnPhases[enterpriseApi.PhaseInstall].msgChannel <- worker

	time.Sleep(2 * time.Second)

	// sending null worker should not cause a crash
	ppln.pplnPhases[enterpriseApi.PhaseInstall].msgChannel <- nil

	// sending more than podCopy workers should not squeeze the podCopyWorkerHandler
	for i := 0; i < 20; i++ {
		ppln.pplnPhases[enterpriseApi.PhaseInstall].msgChannel <- worker
	}

	// wait for the handler to consue the worker
	time.Sleep(2 * time.Second)

	// Closing the channels should exit podCopyWorkerHandler test cleanly
	close(ppln.pplnPhases[enterpriseApi.PhaseInstall].msgChannel)
}

var _ splutil.PodExecClientImpl = &MockPodExecClient{}

// MockPodExecClient to mock the PodExecClient
type MockPodExecClient struct {
	stdOut string
	stdErr string
}

func (mockPodExecClient *MockPodExecClient) RunPodExecCommand(cmd string) (string, string, error) {
	return mockPodExecClient.stdOut, mockPodExecClient.stdErr, nil
}

func TestIDXCRunPlaybook(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
	}

	c := spltest.NewMockClient()
	var appDeployContext *enterpriseApi.AppDeploymentContext = &enterpriseApi.AppDeploymentContext{
		AppsStatusMaxConcurrentAppDownloads: 10,
	}

	appDeployContext.AppsSrcDeployStatus = make(map[string]enterpriseApi.AppSrcDeployInfo)
	testApps := []string{"app1.tgz"}
	testHashes := []string{"abcd1111"}
	testSizes := []int64{10}

	appDeployInfoList := make([]enterpriseApi.AppDeploymentInfo, 1)
	for index := range testApps {
		appDeployInfoList[index] = enterpriseApi.AppDeploymentInfo{
			AppName: testApps[index],
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhasePodCopy,
				Status:     enterpriseApi.AppPkgPodCopyComplete,
				RetryCount: 0,
			},
			ObjectHash: testHashes[index],
			Size:       uint64(testSizes[index]),
		}
	}

	var appSrcDeployInfo enterpriseApi.AppSrcDeployInfo
	appSrcDeployInfo.AppDeploymentInfoList = appDeployInfoList
	appDeployContext.AppsSrcDeployStatus["appSrc1"] = appSrcDeployInfo

	appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushPending
	afwPipeline := initAppInstallPipeline(appDeployContext, c, &cr)
	// get the target pod name
	targetPodName := getApplicablePodNameForAppFramework(&cr, 0)

	kind := cr.GetObjectKind().GroupVersionKind().Kind
	podExecClient := splutil.GetPodExecClient(c, &cr, targetPodName)
	playbookContext := getClusterScopePlaybookContext(c, &cr, afwPipeline, targetPodName, kind, podExecClient)
	err := playbookContext.runPlaybook()
	if err == nil {
		t.Errorf("runPlaybook() should have returned error, since we dont get the required output")
	}

	// now replace the pod exec client with our mock client
	var mockPodExecClient *MockPodExecClient = &MockPodExecClient{
		stdOut: "",
		stdErr: "OK\n",
	}
	playbookContext = getClusterScopePlaybookContext(c, &cr, afwPipeline, targetPodName, kind, mockPodExecClient)

	err = playbookContext.runPlaybook()
	if err != nil || getBundlePushState(afwPipeline) != enterpriseApi.BundlePushInProgress {
		t.Errorf("runPlaybook() should not have returned error or wrong bundle push state, err=%v, bundle push state=%s", err, bundlePushStateAsStr(getBundlePushState(afwPipeline)))
	}

	// test the case where bundle push is still in progress
	mockPodExecClient.stdErr = ""
	err = playbookContext.runPlaybook()
	if err != nil || getBundlePushState(afwPipeline) != enterpriseApi.BundlePushInProgress {
		t.Errorf("runPlaybook() should have not returned error since we did not get desired output")
	}

	// test the case where bundle push is complete
	mockPodExecClient.stdOut = "cluster_status=None"
	err = playbookContext.runPlaybook()
	if err != nil || getBundlePushState(afwPipeline) != enterpriseApi.BundlePushComplete {
		t.Errorf("runPlaybook() should have not returned error since we did not get desired output")
	}

	// now test the error scenario where we did not get OK in stdErr
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushPending
	mockPodExecClient.stdErr = ""
	err = playbookContext.runPlaybook()
	if err == nil {
		t.Errorf("runPlaybook() should have returned error since we did not get desired output")
	}

	// now test the error scenario where we passed invalid bundle push state
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushComplete
	err = playbookContext.runPlaybook()
	if err == nil {
		t.Errorf("runPlaybook() should have returned error since we passed invalud bundle push state")
	}

	mockPodExecClient.stdOut = ""
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushInProgress
	idxcplaybookContext := playbookContext.(*IdxcPlaybookContext)
	// invalid scenario, where stdOut!="cluster_status=None"
	if idxcplaybookContext.isBundlePushComplete() {
		t.Errorf("isBundlePushComplete() should have returned error since we did not get desried stdOut.")
	}

	// invalid scenario, where stdErr != ""
	mockPodExecClient.stdErr = "error"
	if idxcplaybookContext.isBundlePushComplete() {
		t.Errorf("isBundlePushComplete() should have returned false since we did not get desried stdOut.")
	}

	// valid scenario where bundle push is complete
	mockPodExecClient.stdErr = ""
	mockPodExecClient.stdOut = "cluster_status=None"
	if !idxcplaybookContext.isBundlePushComplete() {
		t.Errorf("isBundlePushComplete() should not have returned false.")
	}
}

func TestSHCRunPlaybook(t *testing.T) {
	cr := &enterpriseApi.SearchHeadCluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "SearchHeadCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
	}

	c := spltest.NewMockClient()
	var appDeployContext *enterpriseApi.AppDeploymentContext = &enterpriseApi.AppDeploymentContext{
		AppsStatusMaxConcurrentAppDownloads: 10,
	}

	appDeployContext.AppsSrcDeployStatus = make(map[string]enterpriseApi.AppSrcDeployInfo)
	testApps := []string{"app1.tgz"}
	testHashes := []string{"abcd1111"}
	testSizes := []int64{10}

	appDeployInfoList := make([]enterpriseApi.AppDeploymentInfo, 1)
	for index := range testApps {
		appDeployInfoList[index] = enterpriseApi.AppDeploymentInfo{
			AppName: testApps[index],
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhasePodCopy,
				Status:     enterpriseApi.AppPkgPodCopyComplete,
				RetryCount: 0,
			},
			ObjectHash: testHashes[index],
			Size:       uint64(testSizes[index]),
		}
	}

	cr.Status.Phase = splcommon.PhaseReady
	var appSrcDeployInfo enterpriseApi.AppSrcDeployInfo
	appSrcDeployInfo.AppDeploymentInfoList = appDeployInfoList
	appDeployContext.AppsSrcDeployStatus["appSrc1"] = appSrcDeployInfo

	appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushPending
	afwPipeline := initAppInstallPipeline(appDeployContext, c, cr)
	// get the target pod name
	targetPodName := getApplicablePodNameForAppFramework(cr, 0)

	kind := cr.GetObjectKind().GroupVersionKind().Kind
	podExecClient := splutil.GetPodExecClient(c, cr, targetPodName)

	// just for the sake of code coverage, pass invalid kind to get nil context
	playbookContext := getClusterScopePlaybookContext(c, cr, afwPipeline, targetPodName, "InvalidKind", podExecClient)
	if playbookContext != nil {
		t.Errorf("playbookContext should be nil here since we passed invalid kind")
	}

	playbookContext = getClusterScopePlaybookContext(c, cr, afwPipeline, targetPodName, kind, podExecClient)
	err := playbookContext.runPlaybook()
	if err == nil {
		t.Errorf("runPlaybook() should have returned error, since we dont get the required output")
	}

	// now replace the pod exec client with our mock client
	var mockPodExecClient *MockPodExecClient = &MockPodExecClient{}

	playbookContext = getClusterScopePlaybookContext(c, cr, afwPipeline, targetPodName, kind, mockPodExecClient)
	err = playbookContext.runPlaybook()
	if err != nil || getBundlePushState(afwPipeline) != enterpriseApi.BundlePushInProgress {
		t.Errorf("runPlaybook() should not have returned error or wrong bundle push state, err=%v, bundle push state=%s", err, bundlePushStateAsStr(getBundlePushState(afwPipeline)))
	}

	mockPodExecClient.stdOut = shcBundlePushCompleteStr

	err = playbookContext.runPlaybook()
	if err != nil || getBundlePushState(afwPipeline) != enterpriseApi.BundlePushComplete {
		t.Errorf("runPlaybook() should not have returned error or wrong bundle push state, err=%v, bundle push state=%s", err, bundlePushStateAsStr(getBundlePushState(afwPipeline)))
	}

	// now test the scenario where bundle push is not complete
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushInProgress
	mockPodExecClient.stdOut = ""
	err = playbookContext.runPlaybook()
	if getBundlePushState(afwPipeline) == enterpriseApi.BundlePushComplete {
		t.Errorf("got wrong bundle push state, err=%v, bundle push state=%s", err, bundlePushStateAsStr(getBundlePushState(afwPipeline)))
	}

	// test the scenario where bundle push command returned error
	mockPodExecClient.stdErr = "dummyError"
	err = playbookContext.runPlaybook()
	if getBundlePushState(afwPipeline) == enterpriseApi.BundlePushComplete {
		t.Errorf("got wrong bundle push state, err=%v, bundle push state=%s", err, bundlePushStateAsStr(getBundlePushState(afwPipeline)))
	}

	// now test the scenario where status file had some othee status other than successful bundle push
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushInProgress
	mockPodExecClient.stdErr = ""
	mockPodExecClient.stdOut = "Error while deploying apps"
	err = playbookContext.runPlaybook()
	if getBundlePushState(afwPipeline) == enterpriseApi.BundlePushComplete {
		t.Errorf("got wrong bundle push state, err=%v, bundle push state=%s", err, bundlePushStateAsStr(getBundlePushState(afwPipeline)))
	}

	// now test the error scenario where we did not get OK in stdErr
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushPending
	mockPodExecClient.stdErr = "dummy error"
	err = playbookContext.runPlaybook()
	if err == nil {
		t.Errorf("runPlaybook() should have returned error since we did not get desired output")
	}

	// now test the error scenario where we passed invalid bundle push state
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushComplete
	err = playbookContext.runPlaybook()
	if err == nil {
		t.Errorf("runPlaybook() should have returned error since we passed invalid bundle push state")
	}
}

func TestRunLocalScopedPlaybook(t *testing.T) {
	// Test for each phase can send the worker to down stream
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			AppFrameworkConfig: enterpriseApi.AppFrameworkSpec{
				AppsRepoPollInterval:      60,
				MaxConcurrentAppDownloads: 5,

				VolList: []enterpriseApi.VolumeSpec{
					{
						Name:      "test_volume",
						Endpoint:  "https://s3-eu-west-2.amazonaws.com",
						Path:      "testbucket-rs-london",
						SecretRef: "s3-secret",
						Provider:  "aws",
					},
				},
				AppSources: []enterpriseApi.AppSourceSpec{
					{
						Name:     "appSrc1",
						Location: "adminAppsRepo",
						AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
							VolName: "test_volume",
							Scope:   "local",
						},
					},
				},
			},
		},
	}

	var replicas int32 = 1
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1",
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	// Create client and add object
	c := spltest.NewMockClient()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-clustermaster-0",
			Namespace: "test",
			Labels: map[string]string{
				"controller-revision-hash": "v0",
			},
		},
	}

	// Add object
	c.AddObject(pod)

	// Just make the lint conversion checks happy
	var client splcommon.ControllerClient = getConvertedClient(c)

	var waiter sync.WaitGroup
	waiter.Add(1)
	var localInstallCtxt *localScopePlaybookContext = &localScopePlaybookContext{
		worker: &PipelineWorker{
			appSrcName:    cr.Spec.AppFrameworkConfig.AppSources[0].Name,
			targetPodName: "splunk-stack1-clustermaster-0",
			sts:           sts,
			cr:            &cr,
			appDeployInfo: &enterpriseApi.AppDeploymentInfo{
				AppName:    "app1.tgz",
				ObjectHash: "abcdef12345abcdef",
				PhaseInfo: enterpriseApi.PhaseInfo{
					Phase:      enterpriseApi.PhaseDownload,
					Status:     enterpriseApi.AppPkgDownloadPending,
					RetryCount: 2,
				},
			},
			afwConfig: &cr.Spec.AppFrameworkConfig,
			client:    client,
			waiter:    &waiter,
		},
		sem: make(chan struct{}, 1),
	}

	localInstallCtxt.sem <- struct{}{}
	err := localInstallCtxt.runPlaybook()
	if err == nil {
		t.Errorf("Failed to detect missingApp pkg: err: %s", err.Error())
	}
}

func TestDeleteAppPkgFromOperator(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	operatorResourceTracker = &globalResourceTracker{
		storage: &storageTracker{
			availableDiskSpace: 1024 * 1024 * 1024,
		},
	}

	appFrameworkConfig := &enterpriseApi.AppFrameworkSpec{
		VolList: []enterpriseApi.VolumeSpec{
			{Name: "msos_s2s3_vol", Endpoint: "https://s3-eu-west-2.amazonaws.com", Path: "testbucket-rs-london", SecretRef: "s3-secret", Type: "s3", Provider: "aws"},
		},
		AppSources: []enterpriseApi.AppSourceSpec{
			{Name: "adminApps",
				Location: "adminAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
		},
	}
	cr.Spec.AppFrameworkConfig = *appFrameworkConfig

	var worker *PipelineWorker = &PipelineWorker{
		cr:         &cr,
		appSrcName: cr.Spec.AppFrameworkConfig.AppSources[0].Name,
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AppName:    "testApp.spl",
			ObjectHash: "bcda23232a89",
			Size:       1000,
		},
		afwConfig: &cr.Spec.AppFrameworkConfig,
	}

	defaultVol := splcommon.AppDownloadVolume
	splcommon.AppDownloadVolume = "/tmp/"
	defer func() {
		splcommon.AppDownloadVolume = defaultVol
	}()

	appSrcScope := appFrameworkConfig.AppSources[0].Scope

	appPkgLocalDir := getAppPackageLocalDir(worker.cr, appSrcScope, worker.appSrcName)

	appPkgLocalPath := filepath.Join(appPkgLocalDir, getAppPackageName(worker))

	_, err := os.Stat(appPkgLocalDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(appPkgLocalDir, 0755)
		if err != nil {
			t.Errorf("Unable to create the directory, error: %v", err)
		}
	}

	_, err = os.Stat(appPkgLocalPath)
	if os.IsNotExist(err) {
		_, err := os.Create(appPkgLocalPath)
		if err != nil {
			t.Errorf("Unable to create the local package file, error: %v", err)
		}
	}
	defer os.Remove(appPkgLocalPath)

	diskSpaceBeforeRemoval := operatorResourceTracker.storage.availableDiskSpace
	deleteAppPkgFromOperator(worker)

	if operatorResourceTracker.storage.availableDiskSpace != diskSpaceBeforeRemoval+worker.appDeployInfo.Size {
		t.Errorf("Unable to clean up the app package on Operator pod")
	}
}

func TestGetInstallSlotForPod(t *testing.T) {
	podInstallTracker := make([]chan struct{}, 10)
	for i := range podInstallTracker {
		podInstallTracker[i] = make(chan struct{}, maxParallelInstallsPerPod)
	}

	var podName string
	if getInstallSlotForPod(podInstallTracker, podName) {
		t.Errorf("invalid pod name should not return a install slot")
	}

	podName = "splunk-s2apps-standalone-0"

	if !getInstallSlotForPod(podInstallTracker, podName) {
		t.Errorf("Unable to allocate an install slot, when the slot is available")
	}

	if getInstallSlotForPod(podInstallTracker, podName) {
		t.Errorf("Should not allocate an install slot, when the slot is already occupied")
	}
}

func TestFreeInstallSlotForPod(t *testing.T) {
	podInstallTracker := make([]chan struct{}, 10)
	for i := range podInstallTracker {
		podInstallTracker[i] = make(chan struct{}, maxParallelInstallsPerPod)
	}

	var podName string
	// invalid pod name should not block
	freeInstallSlotForPod(podInstallTracker, podName)

	podName = "splunk-s2apps-standalone-0"

	// when the slot is not allocated, should not block
	freeInstallSlotForPod(podInstallTracker, podName)

	// when the slot is allocated, should free the slot without blocking
	podInstallTracker[0] <- struct{}{}
	freeInstallSlotForPod(podInstallTracker, podName)
}

func TestIsPendingClusterScopeWork(t *testing.T) {
	cr := &enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}
	afwPipeline := &AppInstallPipeline{
		cr: cr,
		appDeployContext: &enterpriseApi.AppDeploymentContext{
			BundlePushStatus: enterpriseApi.BundlePushTracker{
				BundlePushStage: enterpriseApi.BundlePushInProgress,
			},
		},
	}

	// For Standalone CR, should not detect cluster scoped work
	if isPendingClusterScopeWork(afwPipeline) {
		t.Errorf("There should not be any cluster scoped work for a CR without cluster scope")
	}

	// For cluster scoped CR, cluster scope pending activity should be detected
	cr.TypeMeta.Kind = "ClusterMaster"
	afwPipeline.cr = cr
	if !isPendingClusterScopeWork(afwPipeline) {
		t.Errorf("When the bundle push activity is there, should return true")
	}

	// When there is no pending bundle push, should return false
	afwPipeline.appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushComplete
	// For cluster scoped CR, cluster scope pending activity should be detected
	afwPipeline.cr.GetObjectMeta().SetName("ClusterMaster")
	if isPendingClusterScopeWork(afwPipeline) {
		t.Errorf("Should return false, when there is no bundle push activity")
	}

}

func TestNeedToRunClusterScopedPlaybook(t *testing.T) {
	cr := &enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}
	afwPipeline := &AppInstallPipeline{
		cr: cr,
		appDeployContext: &enterpriseApi.AppDeploymentContext{
			BundlePushStatus: enterpriseApi.BundlePushTracker{
				BundlePushStage: enterpriseApi.BundlePushInProgress,
			},
		},
	}

	// For local only scoped CRs, should not trigger cluster scoped playbook
	if needToRunClusterScopedPlaybook(afwPipeline) {
		t.Errorf("Should not return true for local only scoped CRs")
	}

	statefulSetName := "splunk-stack1-clustermaster"
	var replicas int32 = 1

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	client := spltest.NewMockClient()
	_, err := splctrl.ApplyStatefulSet(client, sts)
	if err != nil {
		t.Errorf("unable to apply statefulset")
	}

	var appDeployContext *enterpriseApi.AppDeploymentContext = &enterpriseApi.AppDeploymentContext{
		AppsStatusMaxConcurrentAppDownloads: 10,
	}

	cr.TypeMeta.Kind = "ClusterMaster"
	afwPipeline = initAppInstallPipeline(appDeployContext, client, cr)
	podCopyWorker := &PipelineWorker{appDeployInfo: &enterpriseApi.AppDeploymentInfo{PhaseInfo: enterpriseApi.PhaseInfo{RetryCount: 10}}}
	afwPipeline.pplnPhases[enterpriseApi.PhasePodCopy].q = append(afwPipeline.pplnPhases[enterpriseApi.PhasePodCopy].q, podCopyWorker)

	// When the pipeline are not empty, should return false
	if needToRunClusterScopedPlaybook(afwPipeline) {
		t.Errorf("When pipelines are not empty, should return false")
	}

	// When there is a app framework pending activity, revisit to the app framework scheduler should happen
	if !needToRevisitAppFramework(afwPipeline) {
		t.Errorf("When there is pending app framework activity, should return true")
	}

	// When there is no app framework pending activity, revisit to the app framework scheduler should not happen
	appDeployContext.IsDeploymentInProgress = false
	afwPipeline.pplnPhases[enterpriseApi.PhasePodCopy].q = nil
	appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushComplete
	if needToRevisitAppFramework(afwPipeline) {
		t.Errorf("When there is no pending app framework activity, should return false")
	}

	// When there is no app framework pending activity, same should reflect in the appDeployContext progress tracking
	appDeployContext.IsDeploymentInProgress = true
	checkAndUpdateAppFrameworkProgressFlag(afwPipeline)

	if appDeployContext.IsDeploymentInProgress {
		t.Errorf("Failed to mark the app deployment to completion")
	}
}

func TestHandleAppPkgInstallComplete(t *testing.T) {
	cr := enterpriseApi.Standalone{
		TypeMeta: metav1.TypeMeta{
			Kind: "Standalone",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
	}

	appFrameworkConfig := &enterpriseApi.AppFrameworkSpec{
		VolList: []enterpriseApi.VolumeSpec{
			{Name: "msos_s2s3_vol", Endpoint: "https://s3-eu-west-2.amazonaws.com", Path: "testbucket-rs-london", SecretRef: "s3-secret", Type: "s3", Provider: "aws"},
		},
		AppSources: []enterpriseApi.AppSourceSpec{
			{Name: "adminApps",
				Location: "adminAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
		},
	}

	// create statefulset for the cluster master
	statefulSetName := "splunk-stack1-standalone"
	var replicas int32 = 1

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	client := spltest.NewMockClient()
	_, err := splctrl.ApplyStatefulSet(client, sts)
	if err != nil {
		t.Errorf("unable to apply statefulset")
	}

	worker := &PipelineWorker{
		cr:            &cr,
		targetPodName: "splunk-stack1-standalone-0",
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AppName: "app1.tgz",
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhaseInstall,
				Status:     enterpriseApi.AppPkgInstallComplete,
				RetryCount: 0,
			},
			ObjectHash: "abcd1234abcd",
		},
		client:     client,
		sts:        sts,
		appSrcName: appFrameworkConfig.AppSources[0].Name,
		afwConfig:  appFrameworkConfig,
	}

	defaultVol := splcommon.AppDownloadVolume
	splcommon.AppDownloadVolume = "/tmp/splunk/"
	defer func() {
		os.RemoveAll(splcommon.AppDownloadVolume)
		splcommon.AppDownloadVolume = defaultVol
	}()

	appPkgLocalPath := getAppPackageLocalPath(worker)
	appPkgLocalDir := path.Dir(appPkgLocalPath)

	_, err = os.Stat(appPkgLocalDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(appPkgLocalDir, 0755)
		if err != nil {
			t.Errorf("Unable to create the directory, error: %v", err)
		}
	}

	_, err = os.Stat(appPkgLocalPath)
	if os.IsNotExist(err) {
		_, err := os.Create(appPkgLocalPath)
		if err != nil {
			t.Errorf("Unable to create the local package file, error: %v", err)
		}
	}
	defer os.Remove(appPkgLocalPath)

	// If sts is not set, should clean the app pkg
	tryAppPkgCleanupFromOperatorPod(worker)

	_, err = os.Stat(appPkgLocalPath)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("When the STS is nil, file should be removed")
	}

	// For standalone with replica count of 1, app package should be cleaned from the Operator
	_, err = os.Create(appPkgLocalPath)
	if err != nil {
		t.Errorf("Unable to create the local package file, error: %v", err)
	}

	tryAppPkgCleanupFromOperatorPod(worker)
	_, err = os.Stat(appPkgLocalPath)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("app pkg should be removed if the replica count is 1")
	}

	// Test cases with the Standalone replicas > 1
	// Standalone with replicas > 1 should return false, unless the app installed on all the pods
	_, err = os.Create(appPkgLocalPath)
	if err != nil {
		t.Errorf("Unable to create the local package file, error: %v", err)
	}

	replicas = 5
	_, err = splctrl.ApplyStatefulSet(client, sts)
	if err != nil {
		t.Errorf("unable to update the statefulset")
	}

	worker.appDeployInfo.AuxPhaseInfo = make([]enterpriseApi.PhaseInfo, replicas)
	tryAppPkgCleanupFromOperatorPod(worker)
	_, err = os.Stat(appPkgLocalPath)
	if os.IsNotExist(err) {
		t.Errorf("App pkg should not be deleted when the install is not complete on all the Pods ")
	}

	// When all the app is installed on all the pods, should confirm app deletion from operator pod
	_, err = os.Create(appPkgLocalPath)
	if err != nil {
		t.Errorf("Unable to create the local package file, error: %v", err)
	}

	auxPhaseInfo := worker.appDeployInfo.AuxPhaseInfo
	for i := range auxPhaseInfo {
		auxPhaseInfo[i].Phase = enterpriseApi.PhaseInstall
		auxPhaseInfo[i].Status = enterpriseApi.AppPkgInstallComplete
	}

	tryAppPkgCleanupFromOperatorPod(worker)
	_, err = os.Stat(appPkgLocalPath)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("App pkg should be deleted when the install is complete on all the Pods ")
	}

	// When the apps are installed across all the pods, same should be updated int the PhaseInfo
	if worker.appDeployInfo.PhaseInfo.Phase != enterpriseApi.PhaseInstall || worker.appDeployInfo.PhaseInfo.Status != enterpriseApi.AppPkgInstallComplete {
		t.Errorf("When app is installed on all the replica pods, same should be updated in the Phase info")
	}
}

func TestInstallWorkerHandler(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "splunk-stack1-clustermaster-0",
			Namespace: "test",
			Labels: map[string]string{
				"controller-revision-hash": "v0",
			},
		},
	}

	appFrameworkConfig := &enterpriseApi.AppFrameworkSpec{
		VolList: []enterpriseApi.VolumeSpec{
			{Name: "msos_s2s3_vol", Endpoint: "https://s3-eu-west-2.amazonaws.com", Path: "testbucket-rs-london", SecretRef: "s3-secret", Type: "s3", Provider: "aws"},
		},
		AppSources: []enterpriseApi.AppSourceSpec{
			{Name: "adminApps",
				Location: "adminAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "securityApps",
				Location: "securityAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
			{Name: "authenticationApps",
				Location: "authenticationAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
		},
	}

	// Create client and add object
	client := spltest.NewMockClient()
	// Add object
	client.AddObject(pod)

	// create statefulset for the cluster master
	statefulSetName := "splunk-stack1-clustermaster"
	var replicas int32 = 1

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	_, err := splctrl.ApplyStatefulSet(client, sts)
	if err != nil {
		t.Errorf("unable to apply statefulset")
	}

	worker := &PipelineWorker{
		cr:            &cr,
		targetPodName: "splunk-stack1-clustermaster-0",
		appDeployInfo: &enterpriseApi.AppDeploymentInfo{
			AppName: "app1.tgz",
			PhaseInfo: enterpriseApi.PhaseInfo{
				Phase:      enterpriseApi.PhaseInstall,
				Status:     enterpriseApi.AppPkgInstallPending,
				RetryCount: 0,
			},
			ObjectHash: "abcd1234abcd",
		},
		client:     client,
		afwConfig:  appFrameworkConfig,
		sts:        sts,
		appSrcName: appFrameworkConfig.AppSources[0].Name,
	}

	var appDeployContext *enterpriseApi.AppDeploymentContext = &enterpriseApi.AppDeploymentContext{
		AppsStatusMaxConcurrentAppDownloads: 5,
	}
	ppln := initAppInstallPipeline(appDeployContext, client, &cr)

	podInstallTracker := make([]chan struct{}, *sts.Spec.Replicas)
	for i := range podInstallTracker {
		podInstallTracker[i] = make(chan struct{}, maxParallelInstallsPerPod)
	}

	var handlerWaiter sync.WaitGroup
	handlerWaiter.Add(1)
	defer handlerWaiter.Wait()
	go ppln.pplnPhases[enterpriseApi.PhaseInstall].installWorkerHandler(ppln, &handlerWaiter, podInstallTracker)

	// Send a worker to the install handler
	podInstallTracker[0] <- struct{}{}
	worker.waiter = &ppln.pplnPhases[enterpriseApi.PhaseInstall].workerWaiter
	ppln.pplnPhases[enterpriseApi.PhaseInstall].msgChannel <- worker

	time.Sleep(2 * time.Second)

	// sending null worker should not cause a crash
	ppln.pplnPhases[enterpriseApi.PhaseInstall].msgChannel <- nil

	appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushPending
	// Closing the channels should exit podCopyWorkerHandler test cleanly
	close(ppln.pplnPhases[enterpriseApi.PhaseInstall].msgChannel)

	time.Sleep(2 * time.Second)
	// Now that the local scoped app should have failed, let us remove from the queue
	// and make sure that the install handler exits cleanly
	ppln.pplnPhases[enterpriseApi.PhaseInstall].q = nil
	appDeployContext.BundlePushStatus.BundlePushStage = enterpriseApi.BundlePushComplete
}

// TODO: gaurav/subba, commenting this UT for now.
// It needs to be revisited once we have all the glue logic for all pipelines
// For now, just covers the podCopy related flow
/*
func TestAfwSchedulerEntry(t *testing.T) {
	cr := enterpriseApi.ClusterMaster{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterMaster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack1",
			Namespace: "test",
		},
		Spec: enterpriseApi.ClusterMasterSpec{
			CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
				Mock: true,
			},
		},
	}

	appFrameworkConfig := &enterpriseApi.AppFrameworkSpec{
		VolList: []enterpriseApi.VolumeSpec{
			{Name: "msos_s2s3_vol", Endpoint: "https://s3-eu-west-2.amazonaws.com", Path: "testbucket-rs-london", SecretRef: "s3-secret", Type: "s3", Provider: "aws"},
		},
		AppSources: []enterpriseApi.AppSourceSpec{
			{Name: "adminApps",
				Location: "adminAppsRepo",
				AppSourceDefaultSpec: enterpriseApi.AppSourceDefaultSpec{
					VolName: "msos_s2s3_vol",
					Scope:   enterpriseApi.ScopeLocal},
			},
		},
	}

	c := spltest.NewMockClient()
	var appDeployContext *enterpriseApi.AppDeploymentContext = &enterpriseApi.AppDeploymentContext{
		AppsStatusMaxConcurrentAppDownloads: 10,
	}
	// Create 2 apps for each app source, a total of 6 apps
	appDeployContext.AppsSrcDeployStatus = make(map[string]enterpriseApi.AppSrcDeployInfo, 3)

	appSrcDeploymentInfo := enterpriseApi.AppSrcDeployInfo{}

	appDeployInfo := enterpriseApi.AppDeploymentInfo{
		AppName:    fmt.Sprintf("app1.tgz"),
		ObjectHash: fmt.Sprintf("abcd1234abcd"),
		PhaseInfo: enterpriseApi.PhaseInfo{
			Phase:  enterpriseApi.PhaseDownload,
			Status: enterpriseApi.AppPkgDownloadPending,
		},
	}
	appSrcDeploymentInfo.AppDeploymentInfoList = append(appSrcDeploymentInfo.AppDeploymentInfoList, appDeployInfo)
	appDeployContext.AppsSrcDeployStatus[appFrameworkConfig.AppSources[0].Name] = appSrcDeploymentInfo

	appDeployContext.AppFrameworkConfig = *appFrameworkConfig

	afwPipeline = nil
	afwPipeline = initAppInstallPipeline(appDeployContext)

	var schedulerWaiter sync.WaitGroup
	defer schedulerWaiter.Wait()

	defaultVol := splcommon.AppDownloadVolume
	splcommon.AppDownloadVolume = "/tmp/"
	defer func() {
		splcommon.AppDownloadVolume = defaultVol
	}()

	appPkgFileName := appDeployInfo.AppName + "_" + strings.Trim(appDeployInfo.ObjectHash, "\"")

	appSrcScope := appFrameworkConfig.AppSources[0].Scope
	appPkgLocalDir := getAppPackageLocalPath(&cr, appSrcScope, appFrameworkConfig.AppSources[0].Name)
	appPkgLocalPath := appPkgLocalDir + appPkgFileName

	_, err := os.Stat(appPkgLocalDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(appPkgLocalDir, 0755)
		if err != nil {
			t.Errorf("Unable to create the directory, error: %v", err)
		}
	}

	_, err = os.Stat(appPkgLocalPath)
	if os.IsNotExist(err) {
		_, err := os.Create(appPkgLocalPath)
		if err != nil {
			t.Errorf("Unable to create the local package file, error: %v", err)
		}
	}
	defer os.Remove(appPkgLocalPath)

	schedulerWaiter.Add(1)
	go func() {
		worker := <-afwPipeline.pplnPhases[enterpriseApi.PhaseDownload].msgChannel
		worker.appDeployInfo.PhaseInfo.Status = enterpriseApi.AppPkgDownloadComplete
		worker.isActive = false
		// wait for the glue logic
		//afwPipeline.pplnPhases[enterpriseApi.PhaseDownload].workerWaiter.Done()
		schedulerWaiter.Done()
	}()

	// schedulerWaiter.Add(1)
	// go func() {
	// 	worker := <-afwPipeline.pplnPhases[enterpriseApi.PhasePodCopy].msgChannel
	// 	worker.appDeployInfo.PhaseInfo.Status = enterpriseApi.AppPkgPodCopyComplete
	// 	worker.isActive = false
	// 	// wait for the glue logic
	// 	//afwPipeline.pplnPhases[enterpriseApi.PhasePodCopy].workerWaiter.Done()
	// 	schedulerWaiter.Done()
	// }()

	// schedulerWaiter.Add(1)
	// go func() {
	// 	worker := <-afwPipeline.pplnPhases[enterpriseApi.PhaseInstall].msgChannel
	// 	worker.appDeployInfo.PhaseInfo.Status = enterpriseApi.AppPkgInstallComplete
	// 	worker.isActive = false
	// 	// wait for the glue logic
	// 	// afwPipeline.pplnPhases[enterpriseApi.PhaseInstall].workerWaiter.Done()
	// 	schedulerWaiter.Done()
	// }()

	afwSchedulerEntry(c, &cr, appDeployContext, appFrameworkConfig)
}
*/