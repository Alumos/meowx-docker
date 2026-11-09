//go:build linux

package mesh

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Alumos/meowx-docker/agent/internal/store"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// 喵窝的防火墙（同 OpenWrt 版 firewall_apply）：
//   - 自己的表 inet meowx_mesh：喵窝进来去局域网的做源地址转换（局域网设备的回包回到 NAS）、映射段 1:1 改回原网段、
//     「喵窝里的设备能不能访问 NAS 本身」；内核没有 nf_tables 时改用 iptables-legacy 写同样的规则（firewall_ipt.go）。
//   - Docker 把 IPv4 FORWARD 默认设成丢弃：在 Docker 留给用户的 DOCKER-USER 链里插两条放行（与 iptables 写出来的
//     `-i <网卡> -j ACCEPT` / `-o <网卡> -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT` 同样的表达式，
//     iptables-nft 认得），关掉时只删这两条。Docker 用的是 iptables-legacy 时 nft 里看不到 DOCKER-USER，
//     就用 iptables-legacy 插到 legacy 的那条链里。没有 DOCKER-USER（没装 Docker）时不需要。

const meshTable = "meowx_mesh"

func firewallApply(s store.Mesh, sites []Site) error {
	firewallRemove()
	if err := nftApply(s, sites); err != nil {
		if ierr := iptApply(s, sites); ierr != nil {
			return fmt.Errorf("nftables：%v；iptables-legacy：%v", err, ierr)
		}
	}
	return dockerUserAllow()
}

func nftApply(s store.Mesh, sites []Site) error {
	c, err := nftables.New()
	if err != nil {
		return err
	}
	t := c.AddTable(&nftables.Table{Family: nftables.TableFamilyINet, Name: meshTable})

	post := c.AddChain(&nftables.Chain{Name: "srcnat", Table: t, Type: nftables.ChainTypeNAT,
		Hooknum: nftables.ChainHookPostrouting, Priority: nftables.ChainPriorityNATSource})
	c.AddRule(&nftables.Rule{Table: t, Chain: post, Exprs: []expr.Any{
		&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifname(device)},
		&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1}, &expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: ifname(device)},
		&expr.Masq{},
	}})

	var mapped []Site
	for _, site := range sites {
		if site.Mapped {
			mapped = append(mapped, site)
		}
	}
	if len(mapped) > 0 {
		pre := c.AddChain(&nftables.Chain{Name: "dstnat", Table: t, Type: nftables.ChainTypeNAT,
			Hooknum: nftables.ChainHookPrerouting, Priority: nftables.ChainPriorityNATDest})
		for _, site := range mapped {
			_, adv, err1 := net.ParseCIDR(site.Advertised)
			_, orig, err2 := net.ParseCIDR(site.Original)
			if err1 != nil || err2 != nil || adv.IP.To4() == nil || orig.IP.To4() == nil {
				continue
			}
			c.AddRule(&nftables.Rule{Table: t, Chain: pre, Exprs: []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifname(device)},
				&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.NFPROTO_IPV4}},
				&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4},
				&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: []byte(adv.Mask), Xor: []byte{0, 0, 0, 0}},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte(adv.IP.To4())},
				// dnat ip prefix to <原网段>：主机号不变
				&expr.Immediate{Register: 1, Data: []byte(orig.IP.To4())},
				&expr.Immediate{Register: 2, Data: lastAddr(orig)},
				&expr.NAT{Type: expr.NATTypeDestNAT, Family: unix.NFPROTO_IPV4, RegAddrMin: 1, RegAddrMax: 2, Prefix: true},
			}})
		}
	}

	if !s.HostAccess {
		in := c.AddChain(&nftables.Chain{Name: "input", Table: t, Type: nftables.ChainTypeFilter,
			Hooknum: nftables.ChainHookInput, Priority: nftables.ChainPriorityFilter})
		c.AddRule(&nftables.Rule{Table: t, Chain: in, Exprs: append(ctEstablished(), &expr.Verdict{Kind: expr.VerdictAccept})})
		c.AddRule(&nftables.Rule{Table: t, Chain: in, Exprs: []expr.Any{
			&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifname(device)},
			&expr.Verdict{Kind: expr.VerdictDrop},
		}})
	}
	if err := c.Flush(); err != nil {
		return fmt.Errorf("下发喵窝的 nftables 规则：%w", err)
	}
	return nil
}

func firewallRemove() {
	iptRemove()
	iptDockerUserRemove()
	c, err := nftables.New()
	if err != nil {
		return
	}
	if tables, err := c.ListTablesOfFamily(nftables.TableFamilyINet); err == nil {
		for _, t := range tables {
			if t.Name == meshTable {
				c.DelTable(t)
			}
		}
	}
	if chain := dockerUser(c); chain != nil {
		if rules, err := c.GetRules(chain.Table, chain); err == nil {
			for _, r := range rules {
				if isOurs(r) {
					_ = c.DelRule(r)
				}
			}
		}
	}
	_ = c.Flush()
}

func dockerUser(c *nftables.Conn) *nftables.Chain {
	chains, err := c.ListChainsOfTableFamily(nftables.TableFamilyIPv4)
	if err != nil {
		return nil
	}
	for _, ch := range chains {
		if ch.Name == "DOCKER-USER" && ch.Table.Name == "filter" {
			return ch
		}
	}
	return nil
}

func dockerUserAllow() error {
	// Docker 用 iptables-legacy：DOCKER-USER 在 legacy 表里
	if iptHasDockerUser() {
		if err := iptDockerUserAllow(); err != nil {
			return err
		}
	}
	c, err := nftables.New()
	if err != nil {
		return nil
	}
	chain := dockerUser(c)
	if chain == nil {
		return nil
	}
	// InsertRule 插到最前：后插的在前，所以先插「回包」再插「进来的」
	c.InsertRule(&nftables.Rule{Table: chain.Table, Chain: chain, Exprs: append(append([]expr.Any{
		&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: iptName(device)},
	}, ctEstablished()...), &expr.Counter{}, &expr.Verdict{Kind: expr.VerdictAccept})})
	c.InsertRule(&nftables.Rule{Table: chain.Table, Chain: chain, Exprs: []expr.Any{
		&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: iptName(device)},
		&expr.Counter{}, &expr.Verdict{Kind: expr.VerdictAccept},
	}})
	if err := c.Flush(); err != nil {
		return fmt.Errorf("在 DOCKER-USER 里放行 %s：%w", device, err)
	}
	return nil
}

// isOurs：DOCKER-USER 里匹配 tailscale0 的规则（我们插的）
func isOurs(r *nftables.Rule) bool {
	for i, e := range r.Exprs {
		m, ok := e.(*expr.Meta)
		if !ok || (m.Key != expr.MetaKeyIIFNAME && m.Key != expr.MetaKeyOIFNAME) || i+1 >= len(r.Exprs) {
			continue
		}
		if cmp, ok := r.Exprs[i+1].(*expr.Cmp); ok && bytes.Equal(bytes.TrimRight(cmp.Data, "\x00"), []byte(device)) {
			return true
		}
	}
	return false
}

func ctEstablished() []expr.Any {
	return []expr.Any{
		&expr.Ct{Key: expr.CtKeySTATE, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4,
			Mask: binaryutil.NativeEndian.PutUint32(expr.CtStateBitESTABLISHED | expr.CtStateBitRELATED), Xor: []byte{0, 0, 0, 0}},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: []byte{0, 0, 0, 0}},
	}
}

// ifname：nft 自己的写法（16 字节补零）
func ifname(name string) []byte {
	b := make([]byte, 16)
	copy(b, name)
	return b
}

// iptName：iptables-nft 的写法（名字 + 一个结尾零），在 Docker 的表里与 iptables 写出来的完全一样
func iptName(name string) []byte { return append([]byte(name), 0) }

func lastAddr(n *net.IPNet) []byte {
	ip := binary.BigEndian.Uint32(n.IP.To4())
	ones, _ := n.Mask.Size()
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, ip|(0xffffffff>>ones))
	return out
}

// ---------- 系统能力 ----------

func probe() Checks {
	ch := Checks{Device: device, Port: port, DockerUser: "none"}
	if f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0); err == nil {
		ch.TUN = true
		f.Close()
	}
	if data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		ch.IPForward = strings.TrimSpace(string(data)) == "1"
	}
	if c, err := nftables.New(); err == nil {
		if _, err := c.ListTables(); err == nil {
			ch.NFTables = true
		}
		if dockerUser(c) != nil {
			ch.DockerUser = "nftables"
		}
	}
	if ch.DockerUser == "none" && iptHasDockerUser() {
		ch.DockerUser = "iptables-legacy"
	}
	ch.Firewall = "nftables"
	if !ch.NFTables {
		ch.Firewall = "iptables-legacy"
	}
	return ch
}
