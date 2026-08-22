package api

import (
	"testing"
)

func TestTrafficStopRegistry(t *testing.T) {
	server := newCommandTestServer(nil)
	if server.stopTrafficShape("cmd-x") {
		t.Fatal("未注册的 stop 应为 false")
	}
	stop := server.registerTrafficStop("cmd-x")
	select {
	case <-stop:
		t.Fatal("新注册的 stop channel 不应已关闭")
	default:
	}
	if !server.stopTrafficShape("cmd-x") {
		t.Fatal("已注册的 stop 应为 true")
	}
	select {
	case <-stop:
	default:
		t.Fatal("stopTrafficShape 应关闭 stop channel")
	}
	server.registerTrafficStop("cmd-y")
	server.unregisterTrafficStop("cmd-y")
	if server.stopTrafficShape("cmd-y") {
		t.Fatal("unregister 后 stop 应为 false")
	}
}
