// Copyright (c) 2018-2022 Splunk Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package c3appfw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	enterpriseApi "github.com/splunk/splunk-operator/api/v4"

	"github.com/splunk/splunk-operator/test/testenv"
)

const (
	// PollInterval specifies the polling interval
	PollInterval = 5 * time.Second

	// ConsistentPollInterval is the interval to use to consistently check a state is stable
	ConsistentPollInterval = 200 * time.Millisecond
	ConsistentDuration     = 2000 * time.Millisecond
)

var (
	testenvInstance         *testenv.TestEnv
	testcaseEnvInst         *testenv.TestCaseEnv
	deployment              *testenv.Deployment
	testSuiteName           = "c3appfw-" + testenv.RandomDNSName(3)
	appListV1               []string
	appListV2               []string
	testDataS3Bucket        = os.Getenv("TEST_BUCKET")
	testS3Bucket            = os.Getenv("TEST_INDEXES_S3_BUCKET")
	s3AppDirV1              = testenv.AppLocationV1
	s3AppDirV2              = testenv.AppLocationV2
	s3PVTestApps            = testenv.PVTestAppsLocation
	currDir, _              = os.Getwd()
	downloadDirV1           = filepath.Join(currDir, "c3appfwV1-"+testenv.RandomDNSName(4))
	downloadDirV2           = filepath.Join(currDir, "c3appfwV2-"+testenv.RandomDNSName(4))
	downloadDirPVTestApps   = filepath.Join(currDir, "c3appfwPVTestApps-"+testenv.RandomDNSName(4))
	mc                      *enterpriseApi.MonitoringConsole
	mcName                  string
	appSourceNameMC         string
	s3TestDirMC             string
	cm                      *enterpriseApi.ClusterManager
	shc                     *enterpriseApi.SearchHeadCluster
	resourceVersion         string
	s3TestDirIdxc           string
	s3TestDirShc            string
	indexerReplicas         int
	appSourceVolumeNameIdxc string
	appSourceVolumeNameShc  string
	appVersion              string
	appFileList             []string
	uploadedFiles           []string
	appSourceNameIdxc       string
	appSourceNameShc        string
)

// TestBasic is the main entry point
func TestBasic(t *testing.T) {

	RegisterFailHandler(Fail)

	junitReporter := reporters.NewJUnitReporter(testSuiteName + "_junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Running "+testSuiteName, []Reporter{junitReporter})
}

var _ = BeforeSuite(func() {
	var err error
	ctx := context.TODO()
	testenvInstance, err = testenv.NewDefaultTestEnv(testSuiteName)
	Expect(err).ToNot(HaveOccurred())

	if testenv.ClusterProvider == "eks" {
		// Create a list of apps to upload to S3
		appListV1 = testenv.BasicApps
		appFileList := testenv.GetAppFileList(appListV1)

		// Download V1 Apps from S3
		err = testenv.DownloadFilesFromS3(testDataS3Bucket, s3AppDirV1, downloadDirV1, appFileList)
		Expect(err).To(Succeed(), "Unable to download V1 app files")

		// Create a list of apps to upload to S3 after poll period
		appListV2 = append(appListV1, testenv.NewAppsAddedBetweenPolls...)
		appFileList = testenv.GetAppFileList(appListV2)

		// Download V2 Apps from S3
		err = testenv.DownloadFilesFromS3(testDataS3Bucket, s3AppDirV2, downloadDirV2, appFileList)
		Expect(err).To(Succeed(), "Unable to download V2 app files")

		var err error
		name := fmt.Sprintf("%s-%s", testenvInstance.GetName(), testenv.RandomDNSName(3))
		testcaseEnvInst, err = testenv.NewDefaultTestCaseEnv(testenvInstance.GetKubeClient(), name)
		Expect(err).To(Succeed(), "Unable to create testcaseenv")
		deployment, err = testcaseEnvInst.NewDeployment(testenv.RandomDNSName(3))
		Expect(err).To(Succeed(), "Unable to create deployment")

		// Deploy Monitoring Console
		appVersion := "V1"
		mc, mcName, appSourceNameMC, s3TestDirMC = testenv.SetupMonitoringConsole(ctx, deployment, testcaseEnvInst, appVersion, appListV1, downloadDirV1)

		// Deploy C3 CRD
		cm, shc, resourceVersion, indexerReplicas, s3TestDirIdxc, s3TestDirShc, appSourceVolumeNameIdxc, appSourceVolumeNameShc, appSourceNameIdxc, appSourceNameShc = testenv.SetupC3(ctx, deployment, testcaseEnvInst, appVersion, appListV1, downloadDirV1, mc, mcName)
	} else {
		testenvInstance.Log.Info("Skipping Before Suite Setup", "Cluster Provider", testenv.ClusterProvider)
	}

})

var _ = AfterSuite(func() {
	if testenvInstance != nil {
		Expect(testenvInstance.Teardown()).ToNot(HaveOccurred())
	}

	if testenvInstance != nil {
		Expect(testenvInstance.Teardown()).ToNot(HaveOccurred())
	}

	if deployment != nil {
		deployment.Teardown()
	}

	// Delete locally downloaded app files
	err := os.RemoveAll(downloadDirV1)
	Expect(err).To(Succeed(), "Unable to delete locally downloaded V1 app files")
	err = os.RemoveAll(downloadDirV2)
	Expect(err).To(Succeed(), "Unable to delete locally downloaded V2 app files")
})
