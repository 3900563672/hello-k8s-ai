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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/3900563672/hello-k8s-ai/test/utils"
)

// 部署的命名空间
const namespace = "hello-k8s-ai-system"

// controller 的 ServiceAccount 名称
const serviceAccountName = "hello-k8s-ai-controller-manager"

// metrics 服务名
const metricsServiceName = "hello-k8s-ai-controller-manager-metrics-service"

// 用于读取 metrics 的 RoleBinding 名称
const metricsRoleBindingName = "hello-k8s-ai-metrics-binding"

// Controller Deployment 名称
const controllerDeploymentName = "hello-k8s-ai-controller-manager"

// 放置链路 E2E 使用的 SimulatorInstance 名称
const placementE2EInstanceName = "placement-e2e-instance"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// 每个测试环境准备：创建 namespace，打上安全标签，安装 CRD，部署 controller
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", managerImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

		By("configuring the controller to use the simulator image loaded into Kind")
		cmd = exec.Command(
			"kubectl", "set", "env",
			"deployment/"+controllerDeploymentName,
			"-n", namespace,
			"SIMULATOR_IMAGE="+simulatorImage,
			"SIMULATOR_IMAGE_PULL_POLICY=Never",
		)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to configure the simulator image")
	})

	// 测试结束清理：删除 metrics 的 curl pod，卸载 controller，卸载 CRD，删除 namespace
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// 每个用例失败后收集日志、事件、Pod 描述，方便排查
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// 通过 label 找到 controller-manager pod
				By("getting the name of the controller-manager pod")
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// 确认 pod 状态为 Running
				By("validating the pod's status")
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=hello-k8s-ai-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			// 获取 ServiceAccount 的 token，用于访问 metrics 接口
			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("ensuring the controller pod is ready")
			verifyControllerPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", controllerPodName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller pod not ready")
			}
			Eventually(verifyControllerPodReady, 3*time.Minute, time.Second).Should(Succeed())

			// 等 controller 的日志里出现 "Serving metrics server"，说明 metrics 端点已就绪
			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted, 3*time.Minute, time.Second).Should(Succeed())

			// +kubebuilder:scaffold:e2e-metrics-webhooks-readiness

			// 创建一个 curl pod 直接访问 metrics 端点验证
			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": [
								"for i in $(seq 1 30); do curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics && exit 0 || sleep 2; done; exit 1"
							],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput()
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
			}
			Eventually(verifyMetricsAvailable, 2*time.Minute).Should(Succeed())
		})

		It("创建 Model 时必须提供调度分数", func() {
			const (
				modelName        = "absolute-score-e2e-model"
				tenantName       = "absolute-score-e2e-tenant"
				instanceName     = tenantName + "-" + modelName
				orchestratorName = "absolute-score-e2e-orchestrator"
			)
			cmd := exec.Command(
				"kubectl", "get", "pod", controllerPodName,
				"-n", namespace,
				"-o", "jsonpath={.spec.nodeName}",
			)
			nodeName, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			nodeName = strings.TrimSpace(nodeName)
			Expect(nodeName).NotTo(BeEmpty())

			DeferCleanup(func() {
				cmd := exec.Command(
					"kubectl", "delete",
					"simulatorinstance/"+instanceName,
					"tenantmodelpolicy/absolute-score-e2e-tenant-model",
					"tenantnodepolicy/absolute-score-e2e-tenant-node",
					"modelnodepolicy/absolute-score-e2e-model-node",
					"orchestrator/"+orchestratorName,
					"tenant/"+tenantName,
					"model/"+modelName,
					"workernode/"+nodeName,
					"--ignore-not-found",
				)
				_, _ = utils.Run(cmd)
			})

			By("拒绝没有 spec.absoluteScore 的 Model")
			missingScoreManifest := fmt.Sprintf(`
apiVersion: platform.study.com/v1
kind: Model
metadata:
  name: %s
spec:
  displayName: E2E 模型
  gpuUnits: 1
  maxConcurrency: 1
  coldStartMs: 0
`, modelName)
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(missingScoreManifest)
			_, err = utils.Run(cmd)
			Expect(err).To(HaveOccurred(), "CRD 必须拒绝没有 absoluteScore 的 Model")

			By("接受并保留正数 spec.absoluteScore")
			validManifest := strings.Replace(
				missingScoreManifest,
				"  coldStartMs: 0\n",
				"  absoluteScore: 137\n  coldStartMs: 0\n",
				1,
			)
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(validManifest)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command(
				"kubectl", "get", "model/"+modelName,
				"-o", "jsonpath={.spec.absoluteScore}",
			)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(output)).To(Equal("137"))

			By("通过公开入口创建首次扩容所需的控制面资源")
			controlPlaneManifest := fmt.Sprintf(`
apiVersion: platform.study.com/v1
kind: WorkerNode
metadata:
  name: %s
spec:
  displayName: E2E 节点
  gpu: 8
  maxConcurrency: 8
---
apiVersion: platform.study.com/v1
kind: Tenant
metadata:
  name: %s
spec:
  displayName: E2E 租户
  priority: P3
  qps: 1
  ttftThresholdMs: 500
  queueThreshold: 100
  ttftScaleDownThresholdMs: 200
  queueScaleDownThreshold: 30
---
apiVersion: platform.study.com/v1
kind: TenantNodePolicy
metadata:
  name: absolute-score-e2e-tenant-node
spec:
  tenantRef:
    name: %s
  nodeRef:
    name: %s
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: ModelNodePolicy
metadata:
  name: absolute-score-e2e-model-node
spec:
  modelRef:
    name: %s
  nodeRef:
    name: %s
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: TenantModelPolicy
metadata:
  name: absolute-score-e2e-tenant-model
spec:
  tenantRef:
    name: %s
  modelRef:
    name: %s
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: Orchestrator
metadata:
  name: %s
spec:
  tenantRef:
    name: %s
  scaleUpCooldownSeconds: 0
  scaleDownCooldownSeconds: 0
  minReplicas: 1
  maxReplicas: 2
`, nodeName, tenantName, tenantName, nodeName, modelName, nodeName, tenantName, modelName, orchestratorName, tenantName)
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(controlPlaneManifest)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("等待 Orchestrator 在首次选点中使用 Model 分数")
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "simulatorinstance/"+instanceName,
					"-o", `jsonpath={.spec.replicas}{"|"}{.status.effectiveScore}`,
				)
				result, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(result)).To(Equal("1|137"))
			}).Should(Succeed())

			By("确认首个 Pod 在选定节点就绪")
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "pods",
					"-n", namespace,
					"-l", "platform.study.com/instance="+instanceName,
					"-o", `jsonpath={range .items[*]}{.spec.nodeName}{"|"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}`,
				)
				result, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				pods := utils.GetNonEmptyLines(result)
				g.Expect(pods).To(HaveLen(1))
				g.Expect(strings.TrimSpace(pods[0])).To(Equal(nodeName + "|True"))
			}, 3*time.Minute, time.Second).Should(Succeed())
		})

		It("should schedule a planned replica on the selected node", func() {
			By("reading the node that already runs the controller")
			cmd := exec.Command(
				"kubectl", "get", "pod", controllerPodName,
				"-n", namespace,
				"-o", "jsonpath={.spec.nodeName}",
			)
			nodeName, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			nodeName = strings.TrimSpace(nodeName)
			Expect(nodeName).NotTo(BeEmpty())

			manifest := fmt.Sprintf(`
apiVersion: platform.study.com/v1
kind: TenantNodePolicy
metadata:
  name: placement-e2e-tenant-node
spec:
  tenantRef:
    name: placement-e2e-tenant
  nodeRef:
    name: %s
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: ModelNodePolicy
metadata:
  name: placement-e2e-model-node
spec:
  modelRef:
    name: placement-e2e-model
  nodeRef:
    name: %s
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: SimulatorInstance
metadata:
  name: %s
  annotations:
    platform.study.com/node-placements: '{"version":1,"primaryNode":"%s","placements":[{"nodeName":"%s","replicas":1}]}'
spec:
  tenantRef:
    name: placement-e2e-tenant
  modelRef:
    name: placement-e2e-model
  replicas: 1
  traffic:
    qps: 0
`, nodeName, nodeName, placementE2EInstanceName, nodeName, nodeName)
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(manifest)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				cleanup := exec.Command(
					"kubectl",
					"delete",
					"simulatorinstance/"+placementE2EInstanceName,
					"tenantnodepolicy/placement-e2e-tenant-node",
					"modelnodepolicy/placement-e2e-model-node",
					"--ignore-not-found",
				)
				_, _ = utils.Run(cleanup)
			})

			By("checking the required node affinity materialized by the controller")
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "deployment", "simulator-"+placementE2EInstanceName,
					"-n", namespace,
					"-o", `jsonpath={.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values[0]}{"|"}{.spec.template.spec.securityContext.seccompProfile.type}`,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(Equal(nodeName + "|RuntimeDefault"))
			}).Should(Succeed())

			By("checking the node selected by Kubernetes Scheduler")
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "pods",
					"-n", namespace,
					"-l", "platform.study.com/instance="+placementE2EInstanceName,
					"-o", `jsonpath={range .items[*]}{.spec.nodeName}{"\n"}{end}`,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				podNodes := utils.GetNonEmptyLines(output)
				g.Expect(podNodes).To(HaveLen(1))
				g.Expect(strings.TrimSpace(podNodes[0])).To(Equal(nodeName))
			}).Should(Succeed())

			By("waiting for the simulator pod to become ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "pods",
					"-n", namespace,
					"-l", "platform.study.com/instance="+placementE2EInstanceName,
					"-o", `jsonpath={range .items[*]}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}`,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				readyStates := utils.GetNonEmptyLines(output)
				g.Expect(readyStates).To(HaveLen(1))
				g.Expect(strings.TrimSpace(readyStates[0])).To(Equal("True"))
			}, 3*time.Minute, time.Second).Should(Succeed())
		})

		It("应在不重启 Simulator Pod 的情况下动态应用时间倍速", func() {
			const (
				instanceName = "time-scale-e2e-instance"
				modelName    = "time-scale-e2e-model"
			)
			By("读取可运行 Simulator 的目标节点")
			cmd := exec.Command(
				"kubectl", "get", "pod", controllerPodName,
				"-n", namespace,
				"-o", "jsonpath={.spec.nodeName}",
			)
			nodeName, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			nodeName = strings.TrimSpace(nodeName)
			Expect(nodeName).NotTo(BeEmpty())

			manifest := fmt.Sprintf(`
apiVersion: platform.study.com/v1
kind: SimulationClock
metadata:
  name: default
spec:
  rate: 1
---
apiVersion: platform.study.com/v1
kind: Model
metadata:
  name: %s
spec:
  displayName: 倍速 E2E 模型
  gpuUnits: 1
  maxConcurrency: 1
  absoluteScore: 100
  coldStartMs: 10000
---
apiVersion: platform.study.com/v1
kind: TenantNodePolicy
metadata:
  name: time-scale-e2e-tenant-node
spec:
  tenantRef:
    name: time-scale-e2e-tenant
  nodeRef:
    name: %s
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: ModelNodePolicy
metadata:
  name: time-scale-e2e-model-node
spec:
  modelRef:
    name: %s
  nodeRef:
    name: %s
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: SimulatorInstance
metadata:
  name: %s
  annotations:
    platform.study.com/node-placements: '{"version":1,"primaryNode":"%s","placements":[{"nodeName":"%s","replicas":1}]}'
spec:
  tenantRef:
    name: time-scale-e2e-tenant
  modelRef:
    name: %s
  replicas: 1
  traffic:
    qps: 0
`, modelName, nodeName, modelName, nodeName, instanceName, nodeName, nodeName, modelName)
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(manifest)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				reset := exec.Command(
					"kubectl", "patch", "simulationclock/default", "--type=merge",
					"-p", `{"spec":{"rate":1}}`,
				)
				_, _ = utils.Run(reset)
				cleanup := exec.Command(
					"kubectl", "delete",
					"simulatorinstance/"+instanceName,
					"tenantnodepolicy/time-scale-e2e-tenant-node",
					"modelnodepolicy/time-scale-e2e-model-node",
					"model/"+modelName,
					"--ignore-not-found",
				)
				_, _ = utils.Run(cleanup)
			})

			By("等待 Simulator Pod 就绪并记录 UID")
			var simulatorPodName string
			var simulatorPodUID string
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "pods",
					"-n", namespace,
					"-l", "platform.study.com/instance="+instanceName,
					"-o", `jsonpath={.items[0].metadata.name}{"|"}{.items[0].metadata.uid}{"|"}{.items[0].status.conditions[?(@.type=="Ready")].status}`,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				parts := strings.Split(strings.TrimSpace(output), "|")
				g.Expect(parts).To(HaveLen(3))
				g.Expect(parts[2]).To(Equal("True"))
				simulatorPodName = parts[0]
				simulatorPodUID = parts[1]
			}, 3*time.Minute, time.Second).Should(Succeed())

			By("把 SimulationClock 从 1x 动态调整为 10x")
			cmd = exec.Command(
				"kubectl", "patch", "simulationclock/default", "--type=merge",
				"-p", `{"spec":{"rate":10}}`,
			)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("等待 Controller 同步实例并报告收敛")
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "simulationclock/default",
					"-o", `jsonpath={.spec.rate}{"|"}{.status.appliedRate}{"|"}{.status.synchronizedInstances}{"|"}{.status.totalInstances}{"|"}{.status.conditions[?(@.type=="Ready")].status}`,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				parts := strings.Split(strings.TrimSpace(output), "|")
				g.Expect(parts).To(HaveLen(5))
				g.Expect(parts[0]).To(Equal("10"))
				g.Expect(parts[1]).To(Equal("10"))
				g.Expect(parts[2]).To(Equal(parts[3]))
				g.Expect(parts[3]).NotTo(Equal("0"))
				g.Expect(parts[4]).To(Equal("True"))

				cmd = exec.Command(
					"kubectl", "get", "simulatorinstance/"+instanceName,
					"-o", "jsonpath={.spec.timeScale}",
				)
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(Equal("10"))
			}).Should(Succeed())

			By("确认运行中 Simulator 已读取 10x，且 Pod 没有重建")
			Eventually(func(g Gomega) {
				cmd := exec.Command(
					"kubectl", "get", "--raw",
					fmt.Sprintf(
						"/api/v1/namespaces/%s/pods/%s:9090/proxy/metrics",
						namespace,
						simulatorPodName,
					),
				)
				metricsOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(metricsOutput).To(ContainSubstring("hello_k8s_ai_simulator_time_scale 10"))
				g.Expect(metricsOutput).To(ContainSubstring("hello_k8s_ai_simulator_simulation_step_seconds 50"))

				cmd = exec.Command(
					"kubectl", "get", "pod", simulatorPodName,
					"-n", namespace,
					"-o", "jsonpath={.metadata.uid}",
				)
				uid, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(uid)).To(Equal(simulatorPodUID))
			}, 2*time.Minute, time.Second).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})
})

// serviceAccountToken 通过 TokenRequest API 为指定 ServiceAccount 创建并返回 token。
// 这比手动创建 Secret 更符合 Kubernetes 推荐做法。
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// 将 TokenRequest JSON 写到临时文件
	By("creating temporary file to store the token request")
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		By("executing kubectl command to create the token")
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// 解析返回的 TokenRequest 结构，取出 token
		By("parsing the JSON output to extract the token")
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput 获取 curl-metrics pod 的日志，里面包含了 metrics 端点的响应。
func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	return utils.Run(cmd)
}

// tokenRequest 是 TokenRequest API 响应的简化结构，只取 token 字段。
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
