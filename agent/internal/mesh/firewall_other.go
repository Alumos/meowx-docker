//go:build !linux

package mesh

import (
	"errors"

	"github.com/Alumos/meowx-docker/agent/internal/store"
)

func firewallApply(store.Mesh, []Site) error {
	return errors.New("喵窝的防火墙只在 Linux 上可用")
}

func firewallRemove() {}

func probe() Checks { return Checks{Device: device, Port: port, Firewall: "none", DockerUser: "none"} }
