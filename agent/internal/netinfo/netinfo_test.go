package netinfo

import "testing"

func TestIsVirtual(t *testing.T) {
	for _, n := range []string{"lo", "docker0", "br-1a2b", "veth9", "tailscale0", "meowx-ts", "virbr0"} {
		if !isVirtual(n) {
			t.Errorf("%s 应当算虚拟网卡", n)
		}
	}
	for _, n := range []string{"eth0", "enp4s0", "bond0", "br0", "ovs_eth0"} {
		if isVirtual(n) {
			t.Errorf("%s 不应当算虚拟网卡", n)
		}
	}
}
