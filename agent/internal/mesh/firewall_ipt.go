//go:build linux

package mesh

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/Alumos/meowx-docker/agent/internal/store"
)

// iptables-legacy 版的喵窝防火墙：内核没有 nf_tables 时用（规则与 nft 版一一对应），都在自己的链里，撤销时整链删。
//   nat  MEOWX_MESH_POST（POSTROUTING）：-i <网卡> ! -o <网卡> MASQUERADE
//   nat  MEOWX_MESH_PRE （PREROUTING） ：-i <网卡> -d <映射段> NETMAP --to <原网段>
//   filter MEOWX_MESH_IN（INPUT）      ：已建立的放行，-i <网卡> DROP（不许访问 NAS 本身时）

const iptBin = "iptables-legacy"

var iptChains = []struct{ table, chain, hook string }{
	{"nat", "MEOWX_MESH_POST", "POSTROUTING"},
	{"nat", "MEOWX_MESH_PRE", "PREROUTING"},
	{"filter", "MEOWX_MESH_IN", "INPUT"},
}

func ipt(args ...string) error {
	out, err := exec.Command(iptBin, append([]string{"-w"}, args...)...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s %s：%s", iptBin, strings.Join(args, " "), msg)
	}
	return nil
}

func iptApply(s store.Mesh, sites []Site) error {
	for _, c := range iptChains {
		if err := ipt("-t", c.table, "-N", c.chain); err != nil {
			iptRemove()
			return err
		}
	}
	rules := [][]string{
		{"-t", "nat", "-A", "MEOWX_MESH_POST", "-i", device, "!", "-o", device, "-j", "MASQUERADE"},
	}
	for _, site := range sites {
		if !site.Mapped {
			continue
		}
		_, adv, err1 := net.ParseCIDR(site.Advertised)
		_, orig, err2 := net.ParseCIDR(site.Original)
		if err1 != nil || err2 != nil || adv.IP.To4() == nil || orig.IP.To4() == nil {
			continue
		}
		rules = append(rules, []string{"-t", "nat", "-A", "MEOWX_MESH_PRE", "-i", device, "-d", adv.String(), "-j", "NETMAP", "--to", orig.String()})
	}
	if !s.HostAccess {
		rules = append(rules,
			[]string{"-t", "filter", "-A", "MEOWX_MESH_IN", "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
			[]string{"-t", "filter", "-A", "MEOWX_MESH_IN", "-i", device, "-j", "DROP"})
	}
	for _, c := range iptChains {
		rules = append(rules, []string{"-t", c.table, "-I", c.hook, "-j", c.chain})
	}
	for _, r := range rules {
		if err := ipt(r...); err != nil {
			iptRemove()
			return err
		}
	}
	return nil
}

func iptRemove() {
	if _, err := exec.LookPath(iptBin); err != nil {
		return
	}
	for _, c := range iptChains {
		for ipt("-t", c.table, "-D", c.hook, "-j", c.chain) == nil {
		}
		_ = ipt("-t", c.table, "-F", c.chain)
		_ = ipt("-t", c.table, "-X", c.chain)
	}
}

func iptHasDockerUser() bool {
	if _, err := exec.LookPath(iptBin); err != nil {
		return false
	}
	return ipt("-t", "filter", "-S", "DOCKER-USER") == nil
}

func iptDockerUserRules() [][]string {
	return [][]string{
		{"-o", device, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		{"-i", device, "-j", "ACCEPT"},
	}
}

func iptDockerUserAllow() error {
	iptDockerUserRemove()
	// -I 插到最前：后插的在前，所以先插「回包」再插「进来的」
	for _, r := range iptDockerUserRules() {
		if err := ipt(append([]string{"-t", "filter", "-I", "DOCKER-USER"}, r...)...); err != nil {
			return fmt.Errorf("在 DOCKER-USER 里放行 %s：%w", device, err)
		}
	}
	return nil
}

func iptDockerUserRemove() {
	if !iptHasDockerUser() {
		return
	}
	for _, r := range iptDockerUserRules() {
		for ipt(append([]string{"-t", "filter", "-D", "DOCKER-USER"}, r...)...) == nil {
		}
	}
}
