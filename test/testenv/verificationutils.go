package testenv

import (
	gomega "github.com/onsi/gomega"

	enterprisev1 "github.com/splunk/splunk-operator/pkg/apis/enterprise/v1beta1"
	splcommon "github.com/splunk/splunk-operator/pkg/splunk/common"
)

// StandaloneReady verify Standlone is in ReadyStatus and does not flip-flop
func StandaloneReady(deployment *Deployment, deploymentName string, standalone *enterprisev1.Standalone, testenvInstance *TestEnv) {
	gomega.Eventually(func() splcommon.Phase {
		err := deployment.GetInstance(deploymentName, standalone)
		if err != nil {
			return splcommon.PhaseError
		}
		testenvInstance.Log.Info("Waiting for standalone STATUS to be ready", "instance", standalone.ObjectMeta.Name, "Phase", standalone.Status.Phase)
		DumpGetPods(testenvInstance.GetName())
		return standalone.Status.Phase
	}, deployment.GetTimeout(), PollInterval).Should(gomega.Equal(splcommon.PhaseReady))

	// In a steady state, we should stay in Ready and not flip-flop around
	gomega.Consistently(func() splcommon.Phase {
		_ = deployment.GetInstance(deployment.GetName(), standalone)
		return standalone.Status.Phase
	}, ConsistentDuration, ConsistentPollInterval).Should(gomega.Equal(splcommon.PhaseReady))
}

// SearchHeadClusterReady verify SHC is in READY status and does not flip-flop
func SearchHeadClusterReady(deployment *Deployment, testenvInstance *TestEnv) {
	shc := &enterprisev1.SearchHeadCluster{}
	gomega.Eventually(func() splcommon.Phase {
		err := deployment.GetInstance(deployment.GetName(), shc)
		if err != nil {
			return splcommon.PhaseError
		}
		testenvInstance.Log.Info("Waiting for search head cluster STATUS to be ready", "instance", shc.ObjectMeta.Name, "Phase", shc.Status.Phase)
		DumpGetPods(testenvInstance.GetName())
		return shc.Status.Phase
	}, deployment.GetTimeout(), PollInterval).Should(gomega.Equal(splcommon.PhaseReady))

	// In a steady state, we should stay in Ready and not flip-flop around
	gomega.Consistently(func() splcommon.Phase {
		_ = deployment.GetInstance(deployment.GetName(), shc)
		return shc.Status.Phase
	}, ConsistentDuration, ConsistentPollInterval).Should(gomega.Equal(splcommon.PhaseReady))
}

// VerifyRFSFMet verify RF SF is met on cluster masterr
func VerifyRFSFMet(deployment *Deployment, testenvInstance *TestEnv) {
	gomega.Eventually(func() bool {
		rfSfStatus := CheckRFSF(deployment)
		testenvInstance.Log.Info("Verifying RF SF is met", "Status", rfSfStatus)
		return rfSfStatus
	}, deployment.GetTimeout(), PollInterval).Should(gomega.Equal(true))
}

// LicenseMasterReady verify LM is in ready status and does not flip flop
func LicenseMasterReady(deployment *Deployment, testenvInstance *TestEnv) {
	licenseMaster := &enterprisev1.LicenseMaster{}

	testenvInstance.Log.Info("Verifying License Master becomes READY")
	gomega.Eventually(func() splcommon.Phase {
		err := deployment.GetInstance(deployment.GetName(), licenseMaster)
		if err != nil {
			return splcommon.PhaseError
		}
		testenvInstance.Log.Info("Waiting for License Master instance status to be ready",
			"instance", licenseMaster.ObjectMeta.Name, "Phase", licenseMaster.Status.Phase)
		DumpGetPods(testenvInstance.GetName())

		return licenseMaster.Status.Phase
	}, deployment.GetTimeout(), PollInterval).Should(gomega.Equal(splcommon.PhaseReady))

	// In a steady state, we should stay in Ready and not flip-flop around
	gomega.Consistently(func() splcommon.Phase {
		_ = deployment.GetInstance(deployment.GetName(), licenseMaster)
		return licenseMaster.Status.Phase
	}, ConsistentDuration, ConsistentPollInterval).Should(gomega.Equal(splcommon.PhaseReady))
}

// VerifyLMConfiguredOnPod verify LM is configured on given POD
func VerifyLMConfiguredOnPod(deployment *Deployment, podName string){
	gomega.Eventually(func() bool {
		lmConfigured := CheckLicenseMasterConfigured(deployment, podName)
		return lmConfigured
	}, deployment.GetTimeout(), PollInterval).Should(gomega.Equal(true))
}
