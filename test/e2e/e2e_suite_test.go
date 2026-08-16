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
	// 镜像构建与 CertManager 安装互不依赖：并行执行以缩短 E2E 墙钟。
	// 并行 goroutine 里只执行命令并回传错误，gomega 断言统一在主 goroutine 做。
	needCertManager := os.Getenv("CERT_MANAGER_INSTALL_SKIP") != "true" && !utils.IsCertManagerCRDsInstalled()
	if needCertManager {
		shouldCleanupCertManager = true
	}

	By("并行构建 manager / simulator 镜像并安装 CertManager")
	managerBuild := make(chan error, 1)
	simulatorBuild := make(chan error, 1)
	certManagerInstall := make(chan error, 1)

	go func() {
		cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", managerImage))
		_, err := utils.Run(cmd)
		managerBuild <- err
	}()
	go func() {
		cmd := exec.Command("make", "docker-build-simulator", fmt.Sprintf("SIMULATOR_IMG=%s", simulatorImage))
		_, err := utils.Run(cmd)
		simulatorBuild <- err
	}()
	if needCertManager {
		go func() {
			certManagerInstall <- utils.InstallCertManager()
		}()
	} else {
		certManagerInstall <- nil
	}

	ExpectWithOffset(1, <-managerBuild).NotTo(HaveOccurred(), "Failed to build the manager image")
	ExpectWithOffset(1, <-simulatorBuild).NotTo(HaveOccurred(), "Failed to build the simulator image")
	ExpectWithOffset(1, <-certManagerInstall).NotTo(HaveOccurred(), "Failed to install CertManager")

	// 如果用 Kind 之外的环境跑，需要把镜像推到对应仓库，这里的 LoadImage 可以替换掉
	By("loading the manager image on Kind")
	err := utils.LoadImageToKindClusterWithName(managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	By("loading the simulator image on Kind")
	err = utils.LoadImageToKindClusterWithName(simulatorImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the simulator image into Kind")

	configureKubectlKubeRC()
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

// teardownCertManager 仅卸载本 suite 安装的 CertManager，防止删掉用户自己装的。
func teardownCertManager() {
	if !shouldCleanupCertManager {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling CertManager")
	utils.UninstallCertManager()
}
