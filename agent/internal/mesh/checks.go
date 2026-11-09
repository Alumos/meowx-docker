package mesh

// Checks：喵窝要的系统能力
type Checks struct {
	TUN        bool   `json:"tun"`         // /dev/net/tun 能打开
	IPForward  bool   `json:"ip_forward"`  // net.ipv4.ip_forward = 1（开放局域网网段要）
	NFTables   bool   `json:"nftables"`    // 内核有 nf_tables
	Firewall   string `json:"firewall"`    // 喵窝防火墙用的：nftables / iptables-legacy
	DockerUser string `json:"docker_user"` // Docker 的 DOCKER-USER 链在哪：nftables / iptables-legacy / none
	Device     string `json:"device"`      // 喵窝网卡名
	Port       string `json:"port"`        // 喵窝 UDP 端口
}
