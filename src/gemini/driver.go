package gemini  

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/docker/docker/daemon/execdriver"
	sysinfo "github.com/docker/docker/pkg/system"
)

const (
	DriverName = "gemini"
	Version    = "0.1"
)

type SafeContainer struct {
	pid      int
	sockPath string
	agent    libagent
}

type driver struct {
	root             string
	initPath         string
	activeContainers map[string]*SafeContainer
	machineMemory    int64
	sync.Mutex
}

func NewDriver(root, initPath string, options []string) (*driver, error) {
	meminfo, err := sysinfo.ReadMemInfo()
	if err != nil {
		return nil, err
	}

	if err := sysinfo.MkdirAll(root, 0700); err != nil {
		return nil, err
	}

	return &driver{
		root:             root,
		initPath:         initPath,
		activeContainers: make(map[string]*SafeContainer),
		machineMemory:    meminfo.MemTotal,
	}, nil
}

func RandomMAC() string {
	buf := make([]byte, 3)
	_, err := rand.Read(buf)
	if err != nil {
		log.Error(err)
	}
	return fmt.Sprintf("AA:BB:CC:%02x:%02x:%02x", buf[0], buf[1], buf[2])
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (execdriver.ExitStatus, error) {

	command := "/usr/bin/qemu-system-x86_64"
	mntdir := c.Rootfs[:len(c.Rootfs)-7]
	kernel := "/home/gemini/vmlinux_4_0_4"
	initrd := "/home/gemini/initramfs2.gz"
	sock_path := "/tmp/" + c.ID + ".sock"

	ip, netmask, qemuifup, err := GetIpaddrAndQemuUpScript(c.Network.NamespacePath)
	if err != nil {
		log.Error(err)
	}

	attr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}
	p, err := os.StartProcess(command,
		[]string{
			command,
			"-machine", "pc-i440fx-2.0,usb=off",
			"-global", "kvm-pit.lost_tick_policy=discard",
			"-serial", "pty", "-append", "\"console=ttyS0 panic=1\"",
			"-realtime", "mlock=off", "-no-user-config",
			"-nodefaults", "-no-hpet", "-rtc", "base=utc,driftfix=slew",
			"-no-reboot",
			"-display", "none",
			"-boot", "strict=on", "-m", "128", "-smp", "1",
			"-kernel", kernel,
			"-initrd", initrd,
			"-device", "virtio-serial-pci,id=virtio-serial0,bus=pci.0,addr=0x6",
			"-chardev", "socket,id=charch0,path=" + sock_path + ",server,nowait",
			"-device", "virtserialport,bus=virtio-serial0.0,nr=1,chardev=charch0,id=channel0,name=cvm.channel.0",
			"-fsdev", "local,id=virtio9p,path=" + mntdir + ",security_model=none",
			"-device", "virtio-9p-pci,fsdev=virtio9p,mount_tag=share_dir",
			"-netdev", "type=tap,id=hostnet0,script=" + qemuifup, "-device", "virtio-net-pci,netdev=hostnet0",
		},
		attr)
	if err != nil {
		log.Error(err)
	}

	d.Lock()
	agent := libagent{
		protocol: "unix",
		url:      sock_path,
	}
	d.activeContainers[c.ID] = &SafeContainer{pid: p.Pid,
		sockPath: sock_path,
		agent:    agent}
	d.Unlock()

	// FIXME: wait for sock
	time.Sleep(time.Second * 2)
	err = agent.Init()
	if err != nil {
		log.Errorf("Init error: %s", err)
		return execdriver.ExitStatus{
			ExitCode:  1,
			OOMKilled: false}, nil
	}

	agent.IsReady()

	err = agent.SetIP("eth0", ip, netmask)
	if err != nil {
		log.Errorf("Set ip error: %s", err)
		return execdriver.ExitStatus{
			ExitCode:  1,
			OOMKilled: false}, nil
	}

	err = agent.AddContainer("/cvmfs/rootfs",
		append([]string{c.ProcessConfig.Entrypoint}, c.ProcessConfig.Arguments...),
		c.ProcessConfig.Env)
	if err != nil {
		log.Errorf("Add container error: %s", err)
		return execdriver.ExitStatus{
			ExitCode:  1,
			OOMKilled: false}, nil
	}

	if startCallback != nil {
		pid := p.Pid
		startCallback(&c.ProcessConfig, pid)
	}

	p.Wait()
	return execdriver.ExitStatus{
		ExitCode:  0,
		OOMKilled: false}, nil
}

func (d *driver) Clean(id string) error {
	return os.RemoveAll(filepath.Join(d.root, id))
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	return nil, nil
}

func (d *driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
}

func (d *driver) Kill(c *execdriver.Command, sig int) error {
	d.Lock()
	active := d.activeContainers[c.ID]
	d.Unlock()
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	return syscall.Kill(active.pid, syscall.Signal(sig))
}

func (d *driver) Name() string {
	return fmt.Sprintf("%s-%s", DriverName, Version)
}

func (d *driver) Pause(c *execdriver.Command) error {
	return nil
}

func (d *driver) Unpause(c *execdriver.Command) error {
	return nil
}

func (d *driver) Terminate(c *execdriver.Command) error {
	defer d.cleanContainer(c.ID)
	d.Lock()
	active := d.activeContainers[c.ID]
	d.Unlock()
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	return syscall.Kill(active.pid, 9)
}

func (d *driver) Stats(id string) (*execdriver.ResourceStats, error) {
	return nil, nil
}

func (d *driver) Exec(c *execdriver.Command, processConfig *execdriver.ProcessConfig, pipes *execdriver.Pipes, startCallbak execdriver.StartCallback) (int, error) {
	return 0, nil
}

func (d *driver) cleanContainer(id string) error {
	d.Lock()
	delete(d.activeContainers, id)
	d.Unlock()
	return os.RemoveAll(filepath.Join(d.root, id))
}
