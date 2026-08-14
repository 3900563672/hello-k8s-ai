//go:build e2e
// +build e2e

/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/3900563672/hello-k8s-ai/test/utils"
)

var (
	// managerImage 是用于测试的 manager 镜像名
	managerImage = "example.com/hello-k8s-ai:v0.0.1"
	// simulatorImage 是放置测试实际运行的 Simulator 镜像名
	simulatorImage = "example.com/hello-k8s-ai-simulator:v0.0.1"
	// shouldCleanupCertManager 标记本 suite 是否自己安装了 CertManager，用于后续清理
	shouldCleanupCertManager = false
)

// TestE2E 是 Ginkgo 的入口，跑所有 e2e 用例。
// 默认需要 Kind 集群和 CertManager。
// 可以通过环境变量调整行为：
//
//	KUBECTL_KUBERC=true  启用自定义 kubectl 配置（默认关闭，保证隔离）
//	CERT_MANAGER_INSTALL_SKIP=true  跳过 CertManager 安装
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting hello-k8s-ai e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the manager image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", managerImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	// 如果用 Kind 之外的环境跑，需要把镜像推到对应仓库，这里的 LoadImage 可以替换掉
	By("loading the manager image on Kind")
	err = utils.LoadImageToKindClusterWithName(managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	By("building the simulator image")
	cmd = exec.Command("make", "docker-build-simulator", fmt.Sprintf("SIMULATOR_IMG=%s", simulatorImage))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the simulator image")

	By("loading the simulator image on Kind")
	err = utils.LoadImageToKindClusterWithName(simulatorImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the simulator image into Kind")

	configureKubectlKubeRC()
	setupCertManager()
})

var _ = AfterSuite(func() {
	teardownCertManager()
})

// configureKubectlKubeRC 默认关闭 kubectl 的 kuberc 文件影响，保证测试隔离。
// 如果需要用本地 kubectl 配置，设环境变量 KUBECTL_KUBERC=true。
func configureKubectlKubeRC() {
	if os.Getenv("KUBECTL_KUBERC") != "true" {
		By("disabling kubectl kuberc for test isolation")
		err := os.Setenv("KUBECTL_KUBERC", "false")
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to disable kubectl kuberc")
		_, _ = fmt.Fprintf(GinkgoWriter,
			"kubectl kuberc disabled for consistent test behavior (override with KUBECTL_KUBERC=true)\n")
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "kubectl kuberc enabled (KUBECTL_KUBERC=true)\n")
	}
}

// setupCertManager 如果环境里还没有 CertManager，就装一个。
// 如果 CERT_MANAGER_INSTALL_SKIP=true 或者已经存在则跳过。
func setupCertManager() {
	if os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager installation (CERT_MANAGER_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if CertManager is already installed")
	if utils.IsCertManagerCRDsInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "CertManager is already installed. Skipping installation.\n")
		return
	}

	// 标记为本 suite 安装的，后面 AfterSuite 要卸载掉
	shouldCleanupCertManager = true

	By("installing CertManager")
	Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
}

// teardownCertManager 仅卸载本 suite 安装的 CertManager，防止删掉用户自己装的。
func teardownCertManager() {
	if !shouldCleanupCertManager {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling CertManager")
	utils.UninstallCertManager()
}
