// Package netinfo：宿主机的网卡（容器是 host 网络，看到的就是宿主的）。喵窝页列出可开放的局域网网段用。
package netinfo

import (
	"net"
	"strings"
)

type Host struct {
	Physical []Iface // 物理网卡（有 IPv4 的），局域网入口
	Bridges  []Iface // Docker 网桥（docker0、br-*）
}

type Iface struct {
	Name    string   `json:"name"`
	Subnets []string `json:"subnets"`
	Up      bool     `json:"up"`
}

// 不是局域网入口的虚拟网卡（meowx 前缀含喵窝自己的 meowx-ts）
var virtualPrefixes = []string{"lo", "docker", "br-", "veth", "tun", "tap", "tailscale", "meowx", "virbr", "vnet", "wg", "dummy", "ovs-", "kube", "cni", "flannel", "zt"}

func Discover() Host {
	var h Host
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		up := i.Flags&net.FlagUp != 0
		var subnets []string
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
				_, n, _ := net.ParseCIDR(ipn.String())
				subnets = append(subnets, n.String())
			}
		}
		iface := Iface{Name: i.Name, Subnets: subnets, Up: up}
		switch {
		case i.Name == "docker0" || strings.HasPrefix(i.Name, "br-"):
			h.Bridges = append(h.Bridges, iface)
		case isVirtual(i.Name):
		case len(subnets) > 0:
			h.Physical = append(h.Physical, iface)
		}
	}
	return h
}

func isVirtual(name string) bool {
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// LANSubnets：物理网卡所在的 IPv4 网段
func LANSubnets() []string {
	var out []string
	for _, i := range Discover().Physical {
		out = append(out, i.Subnets...)
	}
	return out
}
