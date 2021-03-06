package gemini  

import (
	"fmt"
	"net"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libcontainer/netlink"
	"github.com/vishvananda/netns"
)

// return ipaddr, netmask, qemu-if-up-script-path, error
func GetIpaddrAndQemuUpScript(namespacePath string) (string, string, string, error) {
	ipaddr := ""
	netmask := ""
	// Save the current network namespace
	origns, _ := netns.Get()
	defer origns.Close()

	container_ns, _ := netns.GetFromPath(namespacePath)
	netns.Set(container_ns)

	// find veth
	var vethInNs net.Interface
	ifaces, _ := net.Interfaces()
	var ipv4 net.Addr
	for _, iface := range ifaces {
		if iface.Name != "lo" {
			vethInNs = iface
			var err error
			ipv4, _, err = GetIfaceAddr(iface.Name)
			if err != nil {
				//return ipaddr, tapid, err
				//continue
				return "", "", "", err
			}
			ipaddr = (ipv4.(*net.IPNet)).IP.String()
			mask := (ipv4.(*net.IPNet)).IP.DefaultMask()
			netmask = fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
			//netmask = (ipv4.(*net.IPNet)).IP.DefaultMask().String()
			break
		}
	}

	// clear IP
	ip, ipnet, err := net.ParseCIDR(ipv4.String())
	if err != nil {
		return "", "", "", err
	}
	if err := netlink.NetworkLinkDelIp(&vethInNs, ip, ipnet); err != nil {
		return "", "", "", err
	}

	// rename interface
	netlink.NetworkLinkDown(&vethInNs)
	if err := netlink.NetworkChangeName(&vethInNs, "veth"+fmt.Sprintf("%d", vethInNs.Index)); err != nil {
		return "", "", "", err
	}

	// remove veth from namespace
	err = netlink.NetworkSetNsPid(&vethInNs, 1)
	if err != nil {
		log.Info("networksetnspid error")
		return "", "", "", err
	}
	// change namespace
	netns.Set(origns)

	// Create a new network bridge
	bridgename := "br-" + fmt.Sprintf("%d", vethInNs.Index)
	err = netlink.CreateBridge(bridgename, true)
	if err != nil {
		return "", "", "", err
	}

	// Bring the bridge up
	br, err := net.InterfaceByName(bridgename)
	netlink.NetworkLinkUp(br)

	// add veth to bridge
	if err := netlink.AddToBridge(&vethInNs, br); err != nil {
		return "", "", "", err
	}
	netlink.NetworkLinkUp(&vethInNs)

	// write qemu-if-up script
	filepath := "/tmp/" + bridgename
	err = writeQemuIfUp(filepath, bridgename)
	if err != nil {
		return "", "", "", err
	}
	return ipaddr, netmask, filepath, nil
}

func writeQemuIfUp(filepath, bridge string) error {
	content := `#!/bin/sh

switch=BRIDGE

if [ -n "$1" ];then
        /sbin/ip link set $1 up
        /usr/sbin/brctl addif $switch $1
        exit 0
else
        echo "Error: no interface specified"
        exit 1
		fi
`
	content = strings.Replace(content, "BRIDGE", bridge, -1)
	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	file.Write([]byte(content))
	file.Close()
	return nil
}

// GetIfaceAddr returns the first IPv4 address and slice of IPv6 addresses for the specified network interface
func GetIfaceAddr(name string) (net.Addr, []net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil, err
	}
	var addrs4 []net.Addr
	var addrs6 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); ip4 != nil {
			addrs4 = append(addrs4, addr)
		} else if ip6 := ip.To16(); len(ip6) == net.IPv6len {
			addrs6 = append(addrs6, addr)
		}
	}
	switch {
	case len(addrs4) == 0:
		return nil, nil, fmt.Errorf("Interface %v has no IPv4 addresses", name)
	case len(addrs4) > 1:
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], addrs6, nil
}
