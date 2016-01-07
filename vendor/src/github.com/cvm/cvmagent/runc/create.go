package runc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/specs"
)

var (
	CreateSuccess = errors.New("OK")
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			logrus.Fatal(err)
		}
		panic("--this line should have never been executed, congratulations--")
	}
}

func CreateContainer(id string, factory libcontainer.Factory, rootfs string, cmdargs []string, env []string) error {
	spec, rspec, err := loadSpec("/config.json", "/runtime.json")
	if err != nil {
		return err
	}

	spec.Root.Path = rootfs
	spec.Process.Args = cmdargs
	spec.Process.Env = env

	errchan := make(chan error)
	go createContainer(factory, id, spec, rspec, errchan)

	err = <-errchan
	if err != CreateSuccess {
		return err
	}
	return nil
}

func createContainer(factory libcontainer.Factory, id string, spec *specs.LinuxSpec, rspec *specs.LinuxRuntimeSpec, errchan chan error) {
	if _, err := startContainer(factory, id, spec, rspec); err != nil {
		errchan <- err
	}
	errchan <- CreateSuccess
}

func startContainer(factory libcontainer.Factory, id string, spec *specs.LinuxSpec, rspec *specs.LinuxRuntimeSpec) (int, error) {
	config, err := createLibcontainerConfig(id, spec, rspec)
	if err != nil {
		return -1, err
	}
	if _, err := os.Stat(config.Rootfs); err != nil {
		if os.IsNotExist(err) {
			return -1, fmt.Errorf("Rootfs (%q) does not exist", config.Rootfs)
		}
		return -1, err
	}
	rootuid, err := config.HostUID()
	if err != nil {
		return -1, err
	}
	container, err := factory.Create(id, config)
	if err != nil {
		return -1, err
	}

	process := newProcess(spec.Process)
	tty, err := newTty(spec.Process.Terminal, process, rootuid)
	if err != nil {
		return -1, err
	}
	handler := newSignalHandler(tty)
	defer handler.Close()
	if err := container.Start(process); err != nil {
		logrus.Errorf("Start Container error: %s", err)
		return -1, err
	}
	go handler.forward(process)
	return 1, nil
}

func destroy(container libcontainer.Container) {
	status, err := container.Status()
	if err != nil {
		logrus.Error(err)
	}
	if status != libcontainer.Checkpointed {
		if err := container.Destroy(); err != nil {
			logrus.Error(err)
		}
	}
}

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(p specs.Process) *libcontainer.Process {
	return &libcontainer.Process{
		Args: p.Args,
		Env:  p.Env,
		// TODO: fix libcontainer's API to better support uid/gid in a typesafe way.
		User:   fmt.Sprintf("%d:%d", p.User.UID, p.User.GID),
		Cwd:    p.Cwd,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// getDefaultID returns a string to be used as the container id based on the
// current working directory of the runc process.  This function panics
// if the cwd is unable to be found based on a system error.
func getDefaultID() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Base(cwd)
}
